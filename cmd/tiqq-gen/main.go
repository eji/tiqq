package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/eji/tiqq/codegen"
	"github.com/eji/tiqq/schema"
)

func main() {
	packageName := flag.String("package", "schema", "generated Go package name")
	inputPath := flag.String("schema", "-", "Schema IR JSON file, or - for stdin")
	outputPath := flag.String("output", "schema_gen.go", "generated Go output file, or - for stdout")
	flag.Parse()

	if err := run(*packageName, *inputPath, *outputPath); err != nil {
		fmt.Fprintln(os.Stderr, "tiqq-gen:", err)
		os.Exit(1)
	}
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
	if outputPath == "-" {
		_, err = os.Stdout.Write(generated)
		return err
	}
	if err := os.WriteFile(outputPath, generated, 0o644); err != nil {
		return fmt.Errorf("write generated source: %w", err)
	}
	return nil
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
