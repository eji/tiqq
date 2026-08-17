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

Self joins require explicit aliases so SQL qualifiers and projection identities
remain distinct:

```go
employee := UserTable.As("employee")
manager := UserTable.As("manager")

j := employee.LeftJoin(
	manager,
	tiqq.On(employee.ManagerID, manager.ID),
)

q := j.Select(
	j.Left().Name,
	j.Right().Name,
)

employeeName := row.Get(j.Left().Name)  // string
managerName := row.Get(j.Right().Name)  // sql.Null[string]
```

The generator emits `As`, `InnerJoin`, and `LeftJoin` alias APIs for
self-referencing foreign keys. Empty or duplicate aliases are rejected before
SQL is built.

This prototype intentionally requires Go 1.27 generic methods. At the time of
writing, Go 1.27 is not yet generally released, so use a Go 1.27 prerelease
toolchain. Execution remains owned by `database/sql`, pgx, or another adapter.

To generate typed Go directly from the live PostgreSQL schema, keep the DSN in
an environment variable and use the `postgres` subcommand:

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

Internally, `introspect/postgres` converts `information_schema` into the Schema
IR in `schema`, which `codegen` then consumes. This intermediate representation
is an implementation boundary, not a required user-managed artifact.

The generator emits typed tables and unambiguous FK-based inner/left joins.
Multiple relations from the same parent require an explicit relation API and
are currently rejected instead of producing ambiguous Go methods.

It can also be used with `go generate` without putting credentials in source or
process arguments:

```go
//go:generate go run github.com/eji/tiqq/cmd/tiqq-gen postgres -dsn-env DATABASE_URL -schema public -package dbschema -output schema_gen.go
```
