package tiqq

import (
	"fmt"
	"strings"
)

// TableRef identifies one table occurrence in a query.
type TableRef struct {
	name    string
	alias   string
	dialect Dialect
}

// NewTableRef constructs a PostgreSQL table reference for manually defined schemas.
// Generated schemas use SchemaInfo.Table to retain their introspection dialect.
func NewTableRef(name string) TableRef { return NewSchemaInfo(PostgreSQL).Table(name) }

func (table TableRef) As(alias string) TableRef {
	if alias == "" {
		panic("tiqq: table alias must not be empty")
	}
	table.alias = alias
	return table
}

func (table TableRef) qualifier() string {
	if table.alias != "" {
		return table.alias
	}
	return table.name
}

// ForeignKey is the generated metadata used for omitted ON clauses.
type ForeignKey struct {
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}

// TableInfo connects a generated concrete column view to query metadata.
type TableInfo[C, NC any] struct {
	ref         TableRef
	required    C
	nullable    NC
	foreignKeys []ForeignKey
}

func NewTableInfo[C, NC any](ref TableRef, required C, nullable NC, foreignKeys []ForeignKey) TableInfo[C, NC] {
	return TableInfo[C, NC]{
		ref: ref, required: required, nullable: nullable,
		foreignKeys: append([]ForeignKey(nil), foreignKeys...),
	}
}

// TableLike is implemented by every generated table definition.
type TableLike[C, NC any] interface {
	TiqqTableInfo() TableInfo[C, NC]
}

// JoinSourceInfo carries the concrete left branch and its SQL source.
type JoinSourceInfo[C any] struct {
	columns C
	source  source
}

// JoinSource is implemented by generated tables and Joined trees.
type JoinSource[C any] interface {
	TiqqJoinSource() JoinSourceInfo[C]
}

func TableJoinSource[C, NC any](table TableInfo[C, NC]) JoinSourceInfo[C] {
	return JoinSourceInfo[C]{
		columns: table.required,
		source:  source{tables: []tableSource{tableSourceOf(table)}},
	}
}

type tableSource struct {
	ref         TableRef
	foreignKeys []ForeignKey
}

type source struct {
	tables []tableSource
	joins  []joinClause
}

type joinClause struct {
	kind       string
	right      tableSource
	conditions []Predicate
}

// Joined is a binary typed JOIN tree.
type Joined[L, R any] struct {
	left   L
	right  R
	source source
}

func (joined Joined[L, R]) Left() L  { return joined.left }
func (joined Joined[L, R]) Right() R { return joined.right }

func (joined Joined[L, R]) TiqqJoinSource() JoinSourceInfo[Joined[L, R]] {
	return JoinSourceInfo[Joined[L, R]]{columns: joined, source: joined.source}
}

// On replaces the inferred condition of the most recently added JOIN.
func (joined Joined[L, R]) On(conditions ...Predicate) Joined[L, R] {
	if len(joined.source.joins) == 0 {
		panic("tiqq: ON requires a JOIN")
	}
	index := len(joined.source.joins) - 1
	joined.source.joins = append([]joinClause(nil), joined.source.joins...)
	joined.source.joins[index].conditions = append([]Predicate(nil), conditions...)
	return joined
}

func LeftJoin[LC, RC, RNC any, LT JoinSource[LC], RT TableLike[RC, RNC]](
	left LT,
	right RT,
) Joined[LC, RNC] {
	leftInfo, rightInfo := left.TiqqJoinSource(), right.TiqqTableInfo()
	return Joined[LC, RNC]{
		left: leftInfo.columns, right: rightInfo.nullable,
		source: leftInfo.source.withJoin("LEFT JOIN", tableSourceOf(rightInfo)),
	}
}

func InnerJoin[LC, RC, RNC any, LT JoinSource[LC], RT TableLike[RC, RNC]](
	left LT,
	right RT,
) Joined[LC, RC] {
	leftInfo, rightInfo := left.TiqqJoinSource(), right.TiqqTableInfo()
	return Joined[LC, RC]{
		left: leftInfo.columns, right: rightInfo.required,
		source: leftInfo.source.withJoin("INNER JOIN", tableSourceOf(rightInfo)),
	}
}

func tableSourceOf[C, NC any](info TableInfo[C, NC]) tableSource {
	return tableSource{ref: info.ref, foreignKeys: append([]ForeignKey(nil), info.foreignKeys...)}
}

func (from source) withJoin(kind string, right tableSource) source {
	return source{
		tables: appendCopy(from.tables, right),
		joins:  appendCopy(from.joins, joinClause{kind: kind, right: right}),
	}
}

func (joined Joined[L, R]) Where(predicates ...Predicate) Query {
	return NewQuery(joined.source).Where(predicates...)
}

func (joined Joined[L, R]) Select(columns ...Selection) Query {
	return NewQuery(joined.source).Select(columns...)
}

func (joined Joined[L, R]) GroupBy(columns ...Selection) Query {
	return NewQuery(joined.source).GroupBy(columns...)
}

func NewTableQuery[C, NC any](table TableLike[C, NC]) Query {
	info := table.TiqqTableInfo()
	return NewQuery(source{tables: []tableSource{tableSourceOf(info)}})
}

func NewQuery(from source) Query { return Query{from: from} }

// Query builds a SELECT while retaining typed projection metadata.
type Query struct {
	from        source
	predicates  []Predicate
	projections []columnRef
	groupBy     []columnRef
	having      []Predicate
}

func (query Query) GroupBy(columns ...Selection) Query {
	query.groupBy = make([]columnRef, len(columns))
	for index, column := range columns {
		query.groupBy[index] = column.selection()
	}
	return query
}

func (query Query) Having(predicates ...Predicate) Query {
	query.having = appendCopy(query.having, predicates...)
	return query
}

func (query Query) Where(predicates ...Predicate) Query {
	query.predicates = appendCopy(query.predicates, predicates...)
	return query
}

func (query Query) Select(columns ...Selection) Query {
	query.projections = make([]columnRef, len(columns))
	for index, column := range columns {
		query.projections[index] = column.selection()
	}
	return query
}

func (query Query) Build() (Statement, error) {
	if len(query.projections) == 0 {
		return Statement{}, fmt.Errorf("tiqq: SELECT requires at least one column")
	}
	if len(query.from.tables) == 0 {
		return Statement{}, fmt.Errorf("tiqq: query requires a source")
	}
	renderer, err := queryRenderer(query.from.tables)
	if err != nil {
		return Statement{}, err
	}
	if err := validateDistinctTableReferences(query.from.tables); err != nil {
		return Statement{}, err
	}
	allowed := queryAllowedColumns(query.from)
	if err := validateColumns("SELECT", query.projections, allowed); err != nil {
		return Statement{}, err
	}
	for _, predicate := range query.predicates {
		columns := predicateColumns(predicate)
		if err := validateColumns("WHERE", columns, allowed); err != nil {
			return Statement{}, err
		}
		if err := validateNoAggregates("WHERE", columns); err != nil {
			return Statement{}, err
		}
	}
	if err := validateColumns("GROUP BY", query.groupBy, allowed); err != nil {
		return Statement{}, err
	}
	for _, predicate := range query.having {
		if err := validateColumns("HAVING", predicateColumns(predicate), allowed); err != nil {
			return Statement{}, err
		}
	}
	if err := validateGrouping(query.projections, query.groupBy, query.having); err != nil {
		return Statement{}, err
	}

	var builder strings.Builder
	builder.WriteString("SELECT ")
	for index, column := range query.projections {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderColumn(renderer, column))
	}
	base := query.from.tables[0]
	builder.WriteString(" FROM ")
	renderTable(renderer, &builder, base.ref)

	args := make([]any, 0, len(query.predicates))
	nextArg := 1
	visible := map[string]bool{base.ref.qualifier(): true}
	for _, join := range query.from.joins {
		builder.WriteByte(' ')
		builder.WriteString(join.kind)
		builder.WriteByte(' ')
		renderTable(renderer, &builder, join.right.ref)
		conditions := join.conditions
		if len(conditions) == 0 {
			var err error
			conditions, err = inferJoinConditions(query.from.tables, visible, join.right)
			if err != nil {
				return Statement{}, err
			}
		}
		joinAllowed := copySet(visible)
		joinAllowed[join.right.ref.qualifier()] = true
		builder.WriteString(" ON ")
		for index, condition := range conditions {
			columns := predicateColumns(condition)
			if err := validateColumns("ON", columns, joinAllowed); err != nil {
				return Statement{}, err
			}
			if err := validateNoAggregates("ON", columns); err != nil {
				return Statement{}, err
			}
			if index > 0 {
				builder.WriteString(" AND ")
			}
			text, values := renderPredicate(renderer, condition, &nextArg)
			builder.WriteString(text)
			args = append(args, values...)
		}
		visible[join.right.ref.qualifier()] = true
	}
	if len(query.predicates) > 0 {
		builder.WriteString(" WHERE ")
		for index, predicate := range query.predicates {
			if index > 0 {
				builder.WriteString(" AND ")
			}
			text, values := renderPredicate(renderer, predicate, &nextArg)
			builder.WriteString(text)
			args = append(args, values...)
		}
	}
	if len(query.groupBy) > 0 {
		builder.WriteString(" GROUP BY ")
		for index, column := range query.groupBy {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(renderColumn(renderer, column))
		}
	}
	if len(query.having) > 0 {
		builder.WriteString(" HAVING ")
		for index, predicate := range query.having {
			if index > 0 {
				builder.WriteString(" AND ")
			}
			text, values := renderPredicate(renderer, predicate, &nextArg)
			builder.WriteString(text)
			args = append(args, values...)
		}
	}
	return newStatement(builder.String(), args, query.projections), nil
}

func validateNoAggregates(clause string, columns []columnRef) error {
	for _, column := range columns {
		if column.aggregate {
			return fmt.Errorf("tiqq: %s does not accept aggregate %s", clause, column.id)
		}
	}
	return nil
}

func validateGrouping(projections, groupBy []columnRef, having []Predicate) error {
	grouped := make(map[string]bool, len(groupBy))
	for _, column := range groupBy {
		if column.aggregate {
			return fmt.Errorf("tiqq: GROUP BY does not accept aggregate %s", column.id)
		}
		grouped[column.id] = true
	}
	hasAggregate := false
	for _, projection := range projections {
		hasAggregate = hasAggregate || projection.aggregate
	}
	for _, predicate := range having {
		for _, column := range predicateColumns(predicate) {
			hasAggregate = hasAggregate || column.aggregate
		}
	}
	if !hasAggregate && len(groupBy) == 0 {
		return nil
	}
	for _, projection := range projections {
		if !projection.aggregate && !grouped[projection.id] {
			return fmt.Errorf("tiqq: SELECT column %s must appear in GROUP BY", projection.id)
		}
	}
	for _, predicate := range having {
		for _, column := range predicateColumns(predicate) {
			if !column.aggregate && !grouped[column.id] {
				return fmt.Errorf("tiqq: HAVING column %s must appear in GROUP BY", column.id)
			}
		}
	}
	return nil
}

// MustBuild builds a statement and panics if validation fails.
func (query Query) MustBuild() Statement {
	statement, err := query.Build()
	if err != nil {
		panic(err)
	}
	return statement
}

func validateDistinctTableReferences(tables []tableSource) error {
	seen := make(map[string]bool, len(tables))
	for _, table := range tables {
		qualifier := table.ref.qualifier()
		if seen[qualifier] {
			return fmt.Errorf("tiqq: table aliases must be distinct")
		}
		seen[qualifier] = true
	}
	return nil
}

func queryRenderer(tables []tableSource) (sqlRenderer, error) {
	renderer, err := rendererFor(tables[0].ref)
	if err != nil {
		return nil, err
	}
	for _, table := range tables[1:] {
		other, otherErr := rendererFor(table.ref)
		if otherErr != nil {
			return nil, otherErr
		}
		if renderer.name() != other.name() {
			return nil, fmt.Errorf("tiqq: cannot combine %s and %s SQL dialects", renderer.name(), other.name())
		}
	}
	return renderer, nil
}

func renderTable(renderer sqlRenderer, builder *strings.Builder, table TableRef) {
	builder.WriteString(renderer.quoteIdentifier(table.name))
	if table.alias != "" {
		builder.WriteString(" AS ")
		builder.WriteString(renderer.quoteIdentifier(table.alias))
	}
}

func queryAllowedColumns(from source) map[string]bool {
	allowed := make(map[string]bool, len(from.tables))
	for _, table := range from.tables {
		allowed[table.ref.qualifier()] = true
	}
	return allowed
}

func validateColumns(clause string, columns []columnRef, allowed map[string]bool) error {
	for _, column := range columns {
		if !allowed[column.qualifier] {
			return fmt.Errorf("tiqq: %s column %s is not in query scope", clause, column.id)
		}
	}
	return nil
}

func inferJoinConditions(tables []tableSource, visible map[string]bool, right tableSource) ([]Predicate, error) {
	var candidates [][]Predicate
	for _, table := range tables {
		if !visible[table.ref.qualifier()] {
			continue
		}
		candidates = append(candidates, relationPredicateGroups(table, right)...)
		candidates = append(candidates, relationPredicateGroups(right, table)...)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("tiqq: JOIN requires ON because no foreign key matches")
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("tiqq: JOIN requires ON because multiple foreign keys match")
	}
	return candidates[0], nil
}

func relationPredicateGroups(child, parent tableSource) [][]Predicate {
	var result [][]Predicate
	for _, foreignKey := range child.foreignKeys {
		if foreignKey.ReferencedTable != parent.ref.name || len(foreignKey.Columns) == 0 || len(foreignKey.Columns) != len(foreignKey.ReferencedColumns) {
			continue
		}
		conditions := make([]Predicate, len(foreignKey.Columns))
		for index, column := range foreignKey.Columns {
			conditions[index] = Predicate{node: predicateNode{
				kind: columnComparison,
				left: columnRef{
					id: child.ref.qualifier() + "." + column, qualifier: child.ref.qualifier(), name: column,
				},
				op: "=",
				rightColumn: columnRef{
					id: parent.ref.qualifier() + "." + foreignKey.ReferencedColumns[index], qualifier: parent.ref.qualifier(), name: foreignKey.ReferencedColumns[index],
				},
			}}
		}
		result = append(result, conditions)
	}
	return result
}

func copySet(values map[string]bool) map[string]bool {
	result := make(map[string]bool, len(values)+1)
	for key, value := range values {
		result[key] = value
	}
	return result
}

func appendCopy[T any](base []T, values ...T) []T {
	out := make([]T, len(base), len(base)+len(values))
	copy(out, base)
	return append(out, values...)
}
