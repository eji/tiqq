# tiqq

`tiqq` is a small typed query builder for Go 1.27. This repository currently
contains the first prototype: typed columns and predicates, explicit SELECT,
LEFT JOIN scope rebinding, PostgreSQL placeholders, projection metadata, and
typed result access.

The prototype supports `InnerJoin` and `LeftJoin`. An inner join keeps both
branches non-nullable; a left join makes columns from its right branch nullable.

## Development environment

Nix with flakes and direnv provide the Go 1.27 RC2 toolchain automatically:

```sh
direnv allow
go version
```

The reported version must start with `go version go1.27rc2`.

Common development commands are available through Make:

```sh
make test
make fmt
make vet
make check
```

Install the repository Git hooks once after cloning:

```sh
make hooks
```

Commits then run Gitleaks against staged changes. Both `pre-commit` and
`gitleaks` are supplied by the locked Nix development shell, and the hook uses
the system executables without downloading another toolchain. To scan the full
Git history manually, run `make secrets`.

```go
j := UserTable.LeftJoin(
	AddressTable,
	tiqq.On(UserTable.ID, AddressTable.UserID),
)

q := j.Where(
	j.Left().ID.Eq(int64(100)),
	j.Right().Address.Like("Tokyo%"),
).Select(
	j.Left().ID,
	j.Left().Name,
	j.Right().Address,
)

stmt := q.Build()
rows, err := db.QueryContext(ctx, stmt.SQL(), stmt.Args()...)
```

After an adapter scans values in projection order, result access preserves the
column's generated type:

```go
row, err := tiqq.NewRow(stmt, id, name, nullableAddress)
id      := row.Get(j.Left().ID)       // int64
name    := row.Get(j.Left().Name)     // string
address := row.Get(j.Right().Address) // sql.Null[string]
```

This prototype intentionally requires Go 1.27 generic methods. At the time of
writing, Go 1.27 is not yet generally released, so use a Go 1.27 prerelease
toolchain. Execution remains owned by `database/sql`, pgx, or another adapter.

The database-derived IR is in `schema`, and `introspect/postgres` fills it from
`information_schema` without choosing a SQL driver. The sample generated API is
in `example/schema`. `tiqq-gen` generates typed tables and unambiguous FK-based
inner/left joins from Schema IR JSON:

```sh
go run ./cmd/tiqq-gen \
  -schema schema.json \
  -package dbschema \
  -output internal/dbschema/schema_gen.go
```

Multiple relations from the same parent require an explicit relation API and
are currently rejected instead of producing ambiguous Go methods.

To introspect PostgreSQL and generate Go directly, keep the DSN in an
environment variable and use the `postgres` subcommand:

```sh
export DATABASE_URL='postgres://user:password@localhost:5432/app'

go run ./cmd/tiqq-gen postgres \
  -dsn-env DATABASE_URL \
  -schema public \
  -package dbschema \
  -output internal/dbschema/schema_gen.go
```

The command verifies the connection, reads the live PostgreSQL catalog,
generates formatted Go, and atomically replaces the output file. A failed
connection, introspection, or generation leaves an existing output untouched.
Use `-output -` to write generated code to stdout.

It can also be used with `go generate` without putting credentials in source or
process arguments:

```go
//go:generate go run github.com/eji/tiqq/cmd/tiqq-gen postgres -dsn-env DATABASE_URL -schema public -package dbschema -output schema_gen.go
```
