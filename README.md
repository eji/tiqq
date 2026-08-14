# tiqq

`tiqq` is a small typed query builder for Go 1.27. This repository currently
contains the first prototype: typed columns and predicates, explicit SELECT,
LEFT JOIN scope rebinding, PostgreSQL placeholders, projection metadata, and
typed result access.

The prototype supports `InnerJoin` and `LeftJoin`. An inner join keeps both
branches non-nullable; a left join makes columns from its right branch nullable.

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
in `example/schema`. A general-purpose generator CLI is intentionally deferred;
the prototype currently proves the generated API and query/result type flow.
