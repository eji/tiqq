# Public API and compatibility

tiqq exposes a small application API together with symbols that must be public
because generated code and dialect subpackages live in separate Go packages.
An exported Go identifier is therefore not always an application-facing API.

## Application API

These are the supported entry points for application code:

- generated table values and their `Select`, `Where`, `GroupBy`, `Insert`,
  `Update`, `Delete`, join, and `As` methods;
- `Column`, `NumericColumn`, `Aggregate`, `Predicate`, `Selection`, `Order`,
  `Query`, and `Joined`, including their query-building methods;
- predicate combinators such as `Eq`, `And`, `Or`, and `Not`;
- `Statement`, `Row`, `Scanner`, `ScanRow`, and `NewRow`;
- `Decimal`;
- the `postgres`, `mysql`, and `sqlite` insert APIs.

`NewTableQuery`, the top-level join functions, and `NewUpdate` and `NewDelete`
are also supported for applications that define table types manually. Most
applications should use generated table methods instead.

## Generator and introspection API

`codegen.Generate` and the `introspect/postgres`, `introspect/mysql`, and
`introspect/sqlite` packages are supported for programs that embed generation.
The types in `schema` are their exchange format.

The JSON representation of `schema` is an implementation-oriented format. It
may gain fields during beta, so persisted schema JSON should be regenerated
rather than treated as a long-lived interchange standard.

## Generated-code API

The following root-package symbols exist primarily for code emitted by
`tiqq-gen`:

- `SchemaInfo`, `TableRef`, `ForeignKey`, `TableInfo`, `TableLike`,
  `JoinSourceInfo`, and `JoinSource`;
- `NewSchemaInfo`, `RequiredColumn`, `NullableColumn`, the numeric-column
  constructors, and `NewTableInfo`;
- `AliasColumn`, the rebind helpers, their numeric variants,
  `RequireDistinctAliases`, and `TableJoinSource`;
- the generic root `InsertQuery`, `InsertDialect`, and dialect marker types.

These symbols are public so generated packages can compile; direct application
use is not the primary contract. Generated source and the tiqq module should
use the same version. After upgrading tiqq, regenerate checked-in schema code
before compiling or committing it.

## Dialect bridge API

`WithConflictDoNothing`, `WithConflictDoUpdate`, `ExcludedAssignment`,
`WithDuplicateKeyDoUpdate`, and `InsertedAssignment` connect the root package
to tiqq's dialect packages. Application code should use `postgres`, `mysql`,
or `sqlite` methods instead. These bridge functions may change together with
the dialect packages without a separate application migration path.

## Beta compatibility policy

During beta:

- patch releases will not intentionally break the application API;
- a minor release may make a necessary breaking change, documented with a
  migration note;
- generated-code and dialect-bridge APIs are version-coupled and may change
  when the generator changes;
- removals from the application API should be deprecated for at least one
  minor release when practical;
- SQL validation may become stricter when accepting an existing query would
  produce invalid or ambiguous SQL.

This policy will be tightened before a v1 release.
