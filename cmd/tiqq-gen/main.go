package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/eji/tiqq/codegen"
	postgresintrospect "github.com/eji/tiqq/introspect/postgres"
	"github.com/eji/tiqq/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tiqq-gen:", err)
		os.Exit(1)
	}
}

func dispatch(arguments []string) error {
	if len(arguments) > 0 && arguments[0] == "postgres" {
		return runPostgresCLI(arguments[1:], defaultPostgresDependencies())
	}
	flags := flag.NewFlagSet("tiqq-gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	packageName := flags.String("package", "schema", "generated Go package name")
	inputPath := flags.String("schema", "-", "Schema IR JSON file, or - for stdin")
	outputPath := flags.String("output", "schema_gen.go", "generated Go output file, or - for stdout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	return run(*packageName, *inputPath, *outputPath)
}

func run(packageName, inputPath, outputPath string) error {
	input, closeInput, err := openInput(inputPath)
	if err != nil {
		return err
	}
	defer closeInput()

	var database schema.Schema
	if err := json.NewDecoder(input).Decode(&database); err != nil {
		return fmt.Errorf("decode schema IR: %w", err)
	}
	generated, err := codegen.Generate(database, codegen.Config{Package: packageName})
	if err != nil {
		return err
	}
	return writeGenerated(outputPath, generated, os.Stdout)
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open schema IR: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}

type postgresDependencies struct {
	lookupEnv  func(string) (string, bool)
	open       func(string) (*sql.DB, error)
	introspect func(context.Context, *sql.DB, string) (schema.Schema, error)
	generate   func(schema.Schema, codegen.Config) ([]byte, error)
	write      func(string, []byte, io.Writer) error
	stdout     io.Writer
}

func defaultPostgresDependencies() postgresDependencies {
	return postgresDependencies{
		lookupEnv: os.LookupEnv,
		open: func(dsn string) (*sql.DB, error) {
			configuration, err := pgx.ParseConfig(dsn)
			if err != nil {
				return nil, fmt.Errorf("invalid PostgreSQL DSN")
			}
			return stdlib.OpenDB(*configuration), nil
		},
		introspect: postgresintrospect.Introspect,
		generate:   codegen.Generate,
		write:      writeGenerated,
		stdout:     os.Stdout,
	}
}

func runPostgresCLI(arguments []string, dependencies postgresDependencies) error {
	flags := flag.NewFlagSet("tiqq-gen postgres", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dsnEnvironment := flags.String("dsn-env", "DATABASE_URL", "environment variable containing the PostgreSQL DSN")
	schemaName := flags.String("schema", "public", "PostgreSQL schema name")
	packageName := flags.String("package", "schema", "generated Go package name")
	outputPath := flags.String("output", "schema_gen.go", "generated Go output file, or - for stdout")
	timeout := flags.Duration("timeout", 10*time.Second, "connection and introspection timeout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	dsn, found := dependencies.lookupEnv(*dsnEnvironment)
	if !found || dsn == "" {
		return fmt.Errorf("PostgreSQL DSN environment variable %s is not set", *dsnEnvironment)
	}
	database, err := dependencies.open(dsn)
	if err != nil {
		return err
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	databaseSchema, err := dependencies.introspect(ctx, database, *schemaName)
	if err != nil {
		return err
	}
	generated, err := dependencies.generate(databaseSchema, codegen.Config{Package: *packageName})
	if err != nil {
		return err
	}
	return dependencies.write(*outputPath, generated, dependencies.stdout)
}

func writeGenerated(path string, generated []byte, stdout io.Writer) error {
	if path == "-" {
		if _, err := stdout.Write(generated); err != nil {
			return fmt.Errorf("write generated source: %w", err)
		}
		return nil
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".tiqq-*.go")
	if err != nil {
		return fmt.Errorf("create temporary generated source: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set generated source permissions: %w", err)
	}
	if _, err := temporary.Write(generated); err != nil {
		return fmt.Errorf("write generated source: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync generated source: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close generated source: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace generated source: %w", err)
	}
	committed = true
	return nil
}
