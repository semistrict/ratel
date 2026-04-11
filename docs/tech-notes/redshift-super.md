# Amazon Redshift `SUPER` Type

Last update: April 2026

This note summarizes Amazon Redshift's `SUPER` type and the surrounding query,
ingestion, and configuration model, based on the AWS Redshift documentation.
It is written for contributors who want a compact implementation-oriented
reference rather than a raw doc dump.

## Primary sources

- [`SUPER type`](https://docs.aws.amazon.com/redshift/latest/dg/r_SUPER_type.html)
- [`Semi-structured data in Amazon Redshift`](https://docs.aws.amazon.com/redshift/latest/dg/super-overview.html)
- [`Querying semi-structured data`](https://docs.aws.amazon.com/redshift/latest/dg/query-super.html)
- [`Loading semi-structured data into Amazon Redshift`](https://docs.aws.amazon.com/redshift/latest/dg/ingest-super.html)
- [`Using COPY to load data into SUPER columns`](https://docs.aws.amazon.com/redshift/latest/dg/copy_json.html)
- [`Operators and functions`](https://docs.aws.amazon.com/redshift/latest/dg/operators-functions.html)
- [`SUPER data type and materialized views`](https://docs.aws.amazon.com/redshift/latest/dg/r_SUPER_MV.html)
- [`enable_case_sensitive_super_attribute`](https://docs.aws.amazon.com/redshift/latest/dg/r_enable_case_sensitive_super_attribute.html)
- [`Limitations`](https://docs.aws.amazon.com/redshift/latest/dg/limitations-super.html)
- [`Using dynamic data masking with SUPER data type paths`](https://docs.aws.amazon.com/redshift/latest/dg/t_ddm-super.html)
- [`JSON functions`](https://docs.aws.amazon.com/redshift/latest/dg/json-functions.html)
- [`Examples of using semi-structured data in Amazon Redshift`](https://docs.aws.amazon.com/redshift/latest/dg/super-examples.html)

## What `SUPER` is

Redshift positions `SUPER` as its native semi-structured type for schemaless
documents and nested values. AWS explicitly recommends using `SUPER` instead of
storing serialized JSON in `VARCHAR`.

The value model is:

- scalar values
  - `null`
  - boolean
  - number
  - string
- complex values
  - arrays
  - objects / structures / tuples

Important size and shape limits:

- one `SUPER` value can be up to `16 MB`
- maximum nesting depth is `1000`
- a single string literal inside one `SUPER` object can be up to `16,000,000`
  bytes

AWS also says the native `SUPER` format is binary and smaller/faster than JSON
text after parsing.

## Storage and schema model

`SUPER` is intentionally schemaless:

- old and new object shapes can coexist in one column
- nested arrays and objects do not need a declared schema before ingestion
- paths are resolved at query time, not fixed at table-definition time

Redshift treats `SUPER` as a first-class type for storing data, but not as a
fully relationally-keyed structure:

- you cannot define a `SUPER` column as a distribution key
- you cannot define a `SUPER` column as a sort key
- AWS recommends shredding frequently queried `SUPER` data into materialized
  views for columnar performance

That is a useful signal: Redshift treats `SUPER` as a convenient storage/query
surface, but still expects hot analytics paths to be normalized or at least
materialized into regular columns.

## Ingestion model

AWS describes two main ingestion paths:

- `JSON_PARSE(...)` for inserting or updating JSON into `SUPER`
- `COPY` for bulk load from external files

Supported bulk formats mentioned across the docs:

- JSON
- Avro
- text
- CSV
- Parquet
- ORC

Additional ingestion details:

- `SUPER` values larger than `1 MB` can only be ingested from:
  - Parquet
  - JSON
  - text
  - CSV
- when using `COPY` from JSON/Avro, the raw JSON object size limit before
  shredding or parsing is `4 MB`
- `COPY ... FORMAT PARQUET|ORC SERIALIZETOJSON` can load nested columnar data
  into `SUPER`
- for ORC, date/time attributes are converted to `varchar` when encoded in
  `SUPER`
- for text/CSV, AWS recommends `ESCAPE` when delimiters may appear inside the
  semi-structured field

## Query model: PartiQL

Redshift uses PartiQL as the query surface over `SUPER`.

Main navigation syntax:

- object navigation: dot notation
  - `data.status`
- array navigation: bracket notation
  - `data.events[0]`
- mixed navigation is normal
  - `data.pnr.events[0].eventType`

AWS explicitly says you can use these path expressions anywhere ordinary column
references can appear, including:

- `SELECT`
- `WHERE`
- `JOIN`
- `GROUP BY`
- `ORDER BY`

### Array iteration / unnest

Redshift supports unnesting arrays in the `FROM` clause in two equivalent ways:

- PartiQL iteration syntax
- `UNNEST`

PartiQL examples:

- `FROM orders, orders.c_orders o`
- `FROM c, c.c_orders o, o.o_lineitems l`

There is also an `AT` form to expose the array index:

- `x AS y AT idx`

### Object unpivot

Redshift supports iterating over object attributes with `UNPIVOT`:

- `UNPIVOT expr AS value_alias AT attribute_alias`

AWS explicitly notes one limitation here:

- correlated unpivoting is not supported

## Dynamic typing

One of the most important semantic choices Redshift makes is dynamic typing for
path expressions over `SUPER`.

The static type of a path expression is effectively `SUPER`, but the dynamic
type is resolved at runtime per row.

Consequences called out by AWS:

- explicit casts are often not required
- joins and groupings can operate directly on path expressions
- the same path can evaluate to different runtime types in different rows

Behavior that matters:

- equality against a mismatched type evaluates to `false`
- ordering/comparison predicates on mismatched types evaluate to `null`
- many functions return `null` when given mistyped `SUPER` arguments
- arrays and objects support deep equality, but AWS warns this can be expensive

Examples of what that means in practice:

- `path = 'x'`
  - `true` if the dynamic value is the string `x`
  - `false` for non-string dynamic values
- `path <= 'x'`
  - meaningful if the dynamic value is a string
  - `null` for non-string dynamic values

AWS positions this as a large reduction in query boilerplate for semi-structured
joins, compared with manual `CASE` + type-checking + casts.

## Lax semantics

Redshift's path navigation is lax by default.

That means invalid navigation produces `null`, not an error.

Examples AWS explicitly documents:

- object navigation returns `null` if:
  - the value is not an object
  - the attribute is missing
- array navigation returns `null` if:
  - the value is not an array
  - the index is out of bounds

This is a major semantic difference from engines that make invalid path access
an error. It pushes `SUPER` toward exploratory and ad hoc querying, but it also
means missing-path behavior can silently blend into regular SQL `NULL`
semantics.

## Case sensitivity

AWS repeatedly recommends enabling case-sensitive handling when working with
`SUPER`.

Relevant settings:

- `enable_case_sensitive_super_attribute`
- `enable_case_sensitive_identifier`

`enable_case_sensitive_super_attribute` controls whether navigation with
non-delimited attribute names is case sensitive.

Behavior from the docs:

- if `true`, unquoted path navigation over `SUPER` attributes is case sensitive
- if `false`, unquoted navigation is not case sensitive
- if the attribute is double-quoted and
  `enable_case_sensitive_identifier = true`, case is preserved regardless of
  the `SUPER`-specific setting

AWS best practice is to set both of these to `true`.

That recommendation appears both on the `SUPER` page and in the semi-structured
overview/examples pages.

### Case sensitivity of table and column names

There is a separate and broader identifier setting:

- `enable_case_sensitive_identifier`

This does **not** just affect `SUPER` paths. It controls whether database,
schema, table, and column identifiers preserve case.

Behavior from the AWS docs:

- if `enable_case_sensitive_identifier = true`
  - double-quoted identifiers preserve case
  - mixed-case table and column names remain distinct
- if `enable_case_sensitive_identifier = false`
  - identifiers are effectively lowercased
  - even double-quoted mixed-case identifiers are treated as lowercase

This is a real footgun. AWS documents the following kind of behavior:

- create a table with two quoted columns that differ only by case
  - `"c"`
  - `"C"`
- query with case sensitivity enabled
  - you see two distinct columns
- query later with case sensitivity disabled
  - both column names collapse to `c`
  - the query result can effectively become ambiguous or surprising

AWS also explicitly says:

- for dot notation over mixed-case identifiers, every case-sensitive identifier
  must be double-quoted
  - for example: `public."MixedCasedTable"."MixedCasedColumn"`

Operationally, the main lesson is:

- if a Redshift deployment uses mixed-case identifiers at all, the
  `enable_case_sensitive_identifier` setting must be treated as part of schema
  semantics, not just session trivia
- AWS recommends pinning it consistently in the parameter group for features
  like autorefresh materialized views, row-level security, and dynamic data
  masking

For contributors comparing Redshift behavior with PostgreSQL-style identifier
rules, this means Redshift's case behavior is more configuration-sensitive than
many users expect. The pain point is not only nested JSON attribute access. It
is also ordinary column-name resolution and how that interacts with quoted DDL
and session settings.

## Operators and functions

AWS documents a dedicated `SUPER` operator/function surface.

A few notable points:

- arithmetic operators work on dynamically typed numeric `SUPER` values
- binary `+` is overloaded:
  - numeric addition for numbers
  - concatenation for strings
- mistyped arithmetic returns `null`
- arithmetic overflow still raises an error

The JSON docs also explicitly recommend this overall lifecycle:

1. ingest JSON text once with `JSON_PARSE`
2. store/query as `SUPER`
3. only serialize back when needed with `JSON_SERIALIZE` or
   `JSON_SERIALIZE_TO_VARBYTE`

This is conceptually similar to "parse once, operate on the binary form".

## Materialized-view guidance

AWS is unusually explicit about performance guidance here:

- ad hoc exploration can run directly over `SUPER`
- frequent analytics should be shredded into materialized views
- materialized views can incrementally refresh
- the point is to exploit Redshift's columnar layout after extracting nested
  paths into normal columns

This is a strong hint that Redshift does not treat direct `SUPER` path access as
the end-state for heavy production analytics, even though it supports it well.

## Security / masking

Redshift supports attaching dynamic data masking policies to `SUPER` paths, but
only for scalar values.

Restrictions documented by AWS:

- masking policies can only target scalar path values
- you cannot mask a complex array/object node directly
- conflicting parent/child paths are not allowed together
- path existence and type are only validated at query runtime, not at attach
  time

That last point is another consequence of dynamic typing and schemaless storage:
validation is deferred until execution.

## Hard limitations

The limitations page is important because it shows where Redshift draws the line
on `SUPER` support.

The documented limitations include:

- no dist key on `SUPER`
- no sort key on `SUPER`
- max object size `16 MB`
- max nesting depth `1000`
- no partial update or partial transform operations on `SUPER` columns
- no right joins or full outer joins with `SUPER` type and its alias
- no XML inbound/outbound serialization for `SUPER`

The "no partial update or transform" limitation is especially important. It
means Redshift does not expose a storage/update model where small mutations to a
large document are guaranteed to stay local. The docs present ingestion/querying
as the mainline path, not in-place structural mutation.

## Practical takeaways

For someone implementing or comparing a semi-structured type, Redshift's
documented shape is roughly:

- store a binary schemaless nested value
- navigate it with SQL-compatible path syntax
- use runtime dynamic typing and lax navigation
- allow array/object iteration in `FROM`
- recommend case-sensitive attribute handling
- recommend parse-once ingestion into the native type
- recommend shredding into materialized views for hot analytics
- do not promise partial-update locality

The most distinctive semantic choices are:

- dynamic typing instead of requiring explicit typed extraction everywhere
- lax semantics instead of hard errors for invalid navigation
- SQL/PartiQL integration directly in `FROM`, `WHERE`, `GROUP BY`, and `JOIN`

The most important implementation boundary AWS publicly documents is:

- querying is rich
- direct hot-path analytics should often be materialized
- mutation locality is not a supported feature

That combination makes `SUPER` feel closer to:

- a queryable nested binary value with permissive semantics

than to:

- a fully normalized nested storage engine with local path updates
