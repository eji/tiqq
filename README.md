# tiqq

`tiqq` is a small typed query builder for Go 1.27. This repository currently
contains the first prototype: typed columns and predicates, explicit SELECT,
JOIN scope rebinding, dialect-aware placeholders, projection metadata, and
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

stmt, err := q.Build()
if err != nil {
	return err
}
rows, err := db.QueryContext(ctx, stmt.SQL(), stmt.Args()...)
```

Common result modifiers remain close to SQL and retain typed ordering keys:

```go
q := UserTable.Select(UserTable.ID, UserTable.Name).
	Distinct().
	OrderBy(UserTable.Name.Asc(), UserTable.ID.Desc()).
	Limit(20).
	Offset(40)
```

Generated tables retain the SQL dialect of their introspected schema. `Build`
uses that metadata for identifier quoting, placeholders, and feature validation;
callers do not pass a dialect or override the schema's source database.

PostgreSQL, MySQL, and SQLite schema IR generate dialect-specific table
metadata and insert builders. MySQL rendering uses backtick-quoted identifiers;
SQLite uses double-quoted identifiers. Both use `?` placeholders.

Generate directly from a PostgreSQL, MySQL, or SQLite database:

```sh
DATABASE_URL='postgres://user:pass@localhost/app' \
  go run ./cmd/tiqq-gen postgres -schema public -package schema -output schema_gen.go

MYSQL_DATABASE_URL='user:pass@tcp(localhost:3306)/app' \
  go run ./cmd/tiqq-gen mysql -database app -package schema -output schema_gen.go

go run ./cmd/tiqq-gen sqlite -database app.db -package schema -output schema_gen.go
```

The PostgreSQL, MySQL, and CGO-free SQLite drivers are used only by the
generator CLI; query execution remains the application's responsibility.
SQLite columns with NUMERIC affinity generate `tiqq.Decimal`; they are never
silently reduced to `float64` by tiqq.

For a statically defined query that should fail fast during development, use
`q.MustBuild()`; it panics with the same validation error.

Updates support type-safe, dynamic `SET` construction. A `WHERE` clause is
required unless `AllRows()` is explicitly selected:

```go
q := UserTable.Update()
if input.Name != nil {
	q = q.Set(UserTable.Name.To(*input.Name))
}
q = q.Where(UserTable.ID.Eq(input.ID))
stmt, err := q.Build()
```

Inserts use the same typed column/value pairing and can append rows for a bulk
insert:

```go
q := UserTable.Insert().Values(
	UserTable.ID.Value(1),
	UserTable.Name.Value("Alice"),
).Values(
	UserTable.ID.Value(2),
	UserTable.Name.Value("Bob"),
)
stmt, err := q.Build()
```

Generated and identity columns are rejected, while omitted required columns
are reported by `Build`.

PostgreSQL-generated insert builders expose PostgreSQL-specific conflict
handling. Other dialects can expose their own methods without adding them to
the common query API:

```go
q := UserTable.Insert().
	Values(UserTable.ID.Value(1), UserTable.Name.Value("Alice")).
	OnConflict(UserTable.ID).
	DoUpdate(postgres.Excluded(UserTable.Name))

stmt, err := q.Build()
```

MySQL-generated insert builders expose `ON DUPLICATE KEY UPDATE`. `Inserted`
refers to the row proposed for insertion using MySQL's row-alias syntax:

```go
q := UserTable.Insert().
	Values(UserTable.ID.Value(1), UserTable.Name.Value("Alice")).
	OnDuplicateKey().
	DoUpdate(mysql.Inserted(UserTable.Name))

stmt, err := q.Build()
```

SQLite-generated builders expose SQLite's `ON CONFLICT`, including its
optional conflict target for the final update clause:

```go
q := UserTable.Insert().
	Values(UserTable.Email.Value("alice@example.com"), UserTable.Name.Value("Alice")).
	OnConflict(UserTable.Email).
	DoUpdate(sqlite.Excluded(UserTable.Name))

stmt, err := q.Build()
```

After an adapter scans values in projection order, result access preserves the
column's generated type:

```go
row, err := tiqq.NewRow(stmt, id, name, nullableAddress)
id, err      := row.Get(j.Left().ID)       // int64
name, err    := row.Get(j.Left().Name)     // string
address, err := row.Get(j.Right().Address) // sql.Null[string]
```

`Get` reports missing projections and driver conversion failures as errors.
`MustGet` is available for queries whose projection is statically fixed.

Numeric aggregate result types follow PostgreSQL's type rules and aggregate
results other than `COUNT` remain nullable:

```go
count := AddressTable.ID.Count() // typed COUNT expression
sum := AddressTable.ID.Sum()     // typed SUM expression
average := AddressTable.ID.Avg() // typed AVG expression

q := AddressTable.
	GroupBy(AddressTable.UserID).
	Having(count.Gt(1)).
	Select(AddressTable.UserID, count, sum, average)

stmt, err := q.Build()
countValue, err := row.Get(count) // int64
sumValue, err := row.Get(sum)     // sql.Null[tiqq.Decimal]
averageValue, err := row.Get(average) // sql.Null[tiqq.Decimal]
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

employeeName, err := row.Get(j.Left().Name) // string
managerName, err := row.Get(j.Right().Name) // sql.Null[string]
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
