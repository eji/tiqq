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

The Nix shell installs the repository pre-commit hook automatically the first
time direnv loads it. Commits then run Gitleaks against staged changes. Both
`pre-commit` and `gitleaks` are supplied by the locked Nix development shell,
and the hook uses the system executables without downloading another toolchain.
To scan the full Git history manually, run `make secrets`.

```go
j := UserTable.LeftJoin(AddressTable).On(
	tiqq.Eq(UserTable.ID, AddressTable.UserID),
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

j := employee.LeftJoin(manager).On(
	tiqq.Eq(employee.ManagerID, manager.ID),
)

q := j.Select(
	j.Left().Name,
	j.Right().Name,
)

employeeName := row.Get(j.Left().Name)  // string
managerName := row.Get(j.Right().Name)  // sql.Null[string]
```

The generator emits one generic `InnerJoin` and `LeftJoin` method per table,
plus `As` for self joins. Empty aliases are rejected immediately and duplicate
aliases are rejected by `Build`.

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

The generator embeds foreign-key metadata in each typed table. A unique
single-column relation lets `Build` infer `ON`:

```go
j := UserTable.LeftJoin(AddressTable)
```

For a join without an FK, with multiple possible FKs, or with extra conditions,
provide the SQL predicates explicitly:

```go
j := UserTable.LeftJoin(AuditLogTable).On(
	tiqq.Eq(UserTable.ID, AuditLogTable.ActorID),
	AuditLogTable.Active.Eq(true),
)
```

Column value and comparison types are checked by the compiler. `Build` checks
that `ON`, `WHERE`, and `SELECT` columns belong to the query.

`ON` and `WHERE` share the same composable predicate API:

```go
query := joined.Where(
	tiqq.And(
		joined.Left().ID.In(10, 20, 30),
		tiqq.Or(
			joined.Left().Name.Like("A%"),
			joined.Right().Address.IsNull(),
		),
		tiqq.Not(joined.Left().Name.Eq("Archived")),
	),
)
```

Available primitives include `And`, `Or`, `Not`, `In`, `NotIn`, `IsNull`, and
`IsNotNull` in addition to typed value and column comparisons. Column-to-column
comparisons use `Eq`, `Ne`, `Lt`, `Lte`, `Gt`, and `Gte`; `Ne` emits the
standard SQL `<>` operator.

Go rejects a recursively growing generic method result as an instantiation
cycle, so subsequent joins use the same top-level constructor that generated
table methods delegate to:

```go
userAddress := UserTable.LeftJoin(AddressTable)
joined := tiqq.LeftJoin(userAddress, CompanyTable)

query := joined.Select(
	joined.Left().Left().ID,
	joined.Left().Right().Address,
	joined.Right().Name,
)
```

The binary type tree and outer-join nullability remain intact even though the
second join cannot use method syntax.

It can also be used with `go generate` without putting credentials in source or
process arguments:

```go
//go:generate go run github.com/eji/tiqq/cmd/tiqq-gen postgres -dsn-env DATABASE_URL -schema public -package dbschema -output schema_gen.go
```
