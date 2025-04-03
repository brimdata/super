## v1.18.0
* Improve the error message shown to a user when a `zed` command is run but there's no pool/branch set for use (#5198)
* Improve the performance of the [`load` operator](docs/language/operators/load.md) by removing an unnecessary/inefficient merge (#5200)
* Improve the [`sort` operator](docs/language/operators/sort.md) to allow different ordering for each key (#5203, #5262)
* Update the [Zeek reference shaper docs](docs/integrations/zeek/shaping-zeek-json.md#reference-shaper-contents) to incorporate changes for [Zeek v7.0.0](https://github.com/zeek/zeek/releases/tag/v7.0.0) logs (#5212)
* Update the [`aggregate` operator docs](docs/language/operators/aggregate.md) to show the use of `by` without an [aggregate function](docs/language/aggregates/_index.md) (#5216)
* Update the [`grok` function docs](docs/language/functions/grok.md) with additional examples and guidance (#5243)
* Update the [Lateral Subquery docs](docs/language/lateral-subqueries.md) with an emphasis on when primitive values or arrays are returned by [Lateral Expressions](docs/language/lateral-subqueries.md#lateral-expressions) (#5264)
* The terms "pipeline" and "branch" are now used throughout the [Zed docs](docs/_index.md) instead of "dataflow" and "leg" (#5272)
* Add docs for [`lake` output format](docs/commands/super.md#superdb-data-lake-metadata-output) and [`zed ls`](docs/commands/super-db.md#ls) (#5187)
* Add docs for the [`top` operator](docs/language/operators/top.md) (#5276)
* Add [`fluentd` integration docs](docs/integrations/fluentd.md) (#5190, #5195)
* Add a [`strftime` function](docs/language/functions/strftime.md) to format `time` values into strings (#5197, #5204)
* Add a `DATESTAMP_ISO8601` [pattern](https://raw.githubusercontent.com/brimdata/super/main/pkg/grok/base.go) for use with the [`grok` function](docs/language/functions/grok.md) (#5232)
* Fix an issue where CSV files without a final newline could not be read (#5185)
* Fix an issue where comparisons against union values produced incorrect results (#5210)
* Fix an issue where concurrent Zed lake operations could cause a service crash (#5279)

## v1.17.0
* Improve the performance of multi-pool searches (e.g., `from * | "MyFilter"`) (#5174)
* Reduce the amount of memory consumed by the [`merge` operator](docs/language/operators/merge.md) and merge-dependent operations, such as compaction (#5171)
* Add the `-pool` flag to [`zed manage`](docs/commands/super-db.md#manage) (#5164)
* Fix an issue where the lake API was not providing query descriptions for Zed programs that contain scopes (#5152)
* Fix an issue where attempts to use the [`load` operator](docs/language/operators/load.md) in `zq` caused a panic (#5162)
* Fix a parser issue with collisions between the names of [user-defined operators](docs/language/statements.md#operator-statements) and [functions](docs/language/statements.md#func-statements) and some built-in [operators](docs/language/operators/_index.md) (#5161)
* Fix an issue where using `null` values in math caused a panic (#5163)

## v1.16.0
* Improve ZNG scanning performance (#5101, #5103)
* Improve the error message shown when `zq` is invoked with a single argument that's not a valid query and doesn't contain a source (#5119)
* Update [Zeek integration docs](docs/integrations/zeek/_index.md), including [reference shaper](docs/integrations/zeek/shaping-zeek-json.md) changes for [Zeek v6.2.0](https://github.com/zeek/zeek/releases/tag/v6.2.0) data (#5106)
* [String literals](docs/language/expressions.md#formatted-string-literals) now use the "f-string" format `f"{ <expr> }"` instead of the previous `${ <expr> }` (#5123)
* Prototype SQL support has been dropped from the Zed language (full SQL support is expected at a later date) (#5109)
* Empty objects and arrays in JSON output are now consistently printed on a single line (#5127)
* Enable colorized ZSON and JSON by default when output is to a terminal (#5114)
* Add JSON pretty print functionality (with color if supported by terminal) when activated via `-J` flag or setting format and the `-pretty` flag (#5096)
* Add a [Grafana integration doc](docs/integrations/grafana.md) that points to the [GitHub repo for the Zed data source plugin for Grafana](https://github.com/brimdata/grafana-zed-datasource) (#5141)
* Fix an issue where stateful expressions inside of [lateral queries](docs/language/lateral-subqueries.md) could generate incorrect results (#5069)
* Fix an issue where stateful expressions inside [user-defined functions](docs/language/statements.md#func-statements) could generate incorrect results (#5093)
* Fix an issue where the [Go library examples](docs/libraries/go.md) failed to compile and run (#5095, #5147)

## v1.15.0
* Improve the error message when reading NDJSON data with an incomplete last line (#5055)
* Improve the error message when math operators are applied to complex types (e.g., attempting to concatenate arrays with `+`) (#5059)
* Improve the error message when `zq` is called with a single argument that's neither a file nor a valid Zed query (#5053)
* Increase the buffer size for `line` input format to 25 MB (#5045, #5048)
* The body of a [user-defined operator](docs/language/statements.md#operator-statements) is now a scope that can include locally-defined statements (#5074)
* The [nameof() function](docs/language/functions/nameof.md) now supports type values (#5081)
* Fix an issue where  [`yield`](docs/language/operators/yield.md) inside a [`switch`](docs/language/operators/switch.md) inside an [`over`](docs/language/operators/over.md) caused a panic (#5058)
* Fix an issue where accessing a variable inside a `switch` inside an `over` inside a `switch` caused a panic (#5080)
* Fix an issue where using `over` in an expression context caused a panic (#5079)
* Fix an issue where math and [`join`](docs/language/operators/join.md) matches involving `float16` and `float32` types could yield incorrect results (#5086)

## v1.14.0
* Add the `-manage` flag to [`zed serve`](docs/commands/super-db.md#serve) to have the Zed service process initiate [maintenance tasks](docs/commands/super-db.md#manage) on a regular interval (#5017)
* Fix an issue where the Python client would not allow loading to a pool with `/` in its name (#5020)
* Fix an issue where pools with KSUID-like names could not be accessed by name (#5019)
* Fix a reference counting issue that could cause a Zed service panic (#5029, #5030)

## v1.13.0
* Improve the error message when [`zed manage -monitor`](docs/commands/super-db.md#manage) is attempted on a local lake (#4979)
* The [`zed serve`](docs/commands/super-db.md#serve) log now includes version, storage root, and auth info at startup (#4988)
* Add [docs for the `zed manage` command](docs/commands/super-db.md#manage) to compact data for improved performance (#4961)
* Add the ability to [cast](docs/language/expressions.md#casts) to Zed's `type` type (#4980, #4985)
* Add the ability to [`yield`](docs/language/operators/yield.md) a Zed `error` literal (#4998)
* Fix an issue with accessing values inside complex literals (#4953)
* Fix an issue where [cast](docs/language/expressions.md#casts) of an empty string to a `duration` value incorrectly yielded `0s` (#4965)
* Fix an issue where a [`zed vacuum`](docs/commands/super-db.md#vacuum) on a large amount of data could crash the Zed service (#4974)
* Fix an issue where some IPv6 values of Zed's `net` type were not parsed correctly in Zed queries (#4992)
* Fix an issue where output of certain union-typed values was not consistent (#4995)
* Fix an issue where parsing of `type` literals inside of `type` literals was incorrectly permitted (#4996)
* Fix an issue where the [`grok()` function](docs/language/functions/grok.md) would panic when encountering optional fields (#5010)

## v1.12.0
* Zed CSV output now handles records with the same field names but different types (#4889)
* The [`grep()` function](docs/language/functions/grep.md) now accepts variables as pattern arguments (#4887)
* Prevent the creation of multi-key pools in Zed lakes (support for this will be added later) (#4903)
* Zed CLI help can now be invoked via `--help` and is printed to stdout instead of stderr (#4907)
* Symbols (e.g., `const` and `type`) may no longer be redefined within the same scope (#4930)
* [Set types](docs/formats/data-model.md#23-set) can now be [sliced](docs/language/expressions.md#slices) (#4939)
* Canceled client requests to the Zed lake API are now logged at `info` level without stack traces (#4947)
* Add support for [TSV](https://en.wikipedia.org/wiki/Tab-separated_values) input/output (useful for spreadsheet data) (#4891, #4913, #4912)
* Add a [`grok()` function](docs/language/functions/grok.md) for parsing text lines (such as logs) into records (#4827)
* Fix an issue where [`switch` operators](docs/language/operators/switch.md) could not be chained back-to-back in a Zed pipeline (#4926)
* Fix a panic that occurred if the [`from` operator](docs/language/operators/from.md) was attempted in Zed run with `zq` (#4936)
* Fix an "invalid union tag" panic that could occur when using the [`collect()` aggregate function](docs/language/aggregates/collect.md) (#4932)

## v1.11.1
* Fix an issue where loading and querying certain data caused a panic (#4877)

## v1.11.0
* Introduce new logic for the `zed` CLI command to [locate the lake](docs/commands/super-db.md#locating-the-lake) (#4758, #4787, #4811)
* [Cast expressions](docs/language/expressions.md#casts) now behave more like function calls (#4805)
* Reduce the amount of memory needed to store a Zed value (#4812)
* Add support for unicode in keywords and identifiers (#4799, #4796)
* Add support for dynamic field paths in "left-hand side" contexts with [`put`](docs/language/operators/put.md), [`cut`](docs/language/operators/cut.md), and [`rename`](docs/language/operators/rename.md) operators (#4795, #4802, #4832)
* Add a [`map()` function](docs/language/functions/map.md) that applies a function to each element of an array or set. To avoid a naming clash, the aggregate function previously called `map()` is now called [`collect_map()`](docs/language/aggregates/collect_map.md) (#4806, #4856, #4860)
* Fix an issue where a [record expression](docs/language/expressions.md#record-expressions) with duplicate fields caused a panic (#4789)
* Fix an issue where valid ZNG input might not be read successfully via auto-detection (#4790)
* Fix an issue where `null` values could cause [`join`](docs/language/operators/join.md) to produce incorrect output (#4801)
* Fix a `zq` where a call to a [user-defined operator](docs/language/statements.md#operator-statements) included via `-I` could fail (#4808)
* Fix an issue where running `zed` commands on a system with many CPU cores caused an internal error (#4826)
* Fix an encoding issue that caused different [map values](docs/formats/data-model.md#24-map) to be rendered the same in Zui (#4853)

## v1.10.0
* Sorting is now performed automatically on [`join`](docs/language/operators/join.md) inputs when needed (explicit [`sort`](docs/language/operators/sort.md) no longer required) (#4770)
* Various query performance improvements (#4736, #4737, #4739, #4740, #4783, #4785)
* [`join`](docs/language/operators/join.md) now works correctly when data inputs are sorted in descending order (#4767)
* Reduce memory consumption during [`delete -where`](docs/commands/super-db.md#delete) operations (#4734)
* Fix a `null`-handling issue that caused incorrect query results after pool compaction (#4735, #4753)
* Allow writing of vectors when compacting objects in a pool (#4756, #4757)
* Ensure query runtime errors are logged and made available through a new [Query Status](docs/lake/api.md#query-status) lake API endpoint (#4763, #4765, #4766, #4769)
* Add an example to the [`where` docs](docs/language/operators/where.md) showing inverse containment logic (#4761)
* Add an example to the [`cut` docs](docs/language/operators/cut.md) that includes setting a default value for a field (#4773, #4776)
* Boolean `not` and `!` now both work the same in [expressions](docs/language/expressions.md#logic) and [search expressions](docs/language/search-expressions.md#boolean-logic) (#4768)
* The [`zed` command](docs/commands/super-db.md) now returns a hint mentioning [`init`](docs/commands/super-db.md#init) if no lake exists at the expected path (#4786)
## v1.9.0
* The [Zed Language Overview docs](docs/language/overview.md) have been split into multiple sections (#4576)
* Add support for [user-defined operators](docs/language/statements.md#operator-statements) (#4417, #4635, #4646, #4644, #4663, #4674, #4698, #4702, #4716)
* Add experimental support to the [`get` operator](docs/language/operators/get.md) for customized methods, headers, and body (#4572)
* Allow float decorators on integers in [ZSON](docs/formats/sup.md) (#4654)
* The [shaping docs](docs/language/shaping.md) have been expanded with a new section on [error handling](docs/language/shaping.md#error-handling) (#4686)
* `zq` no longer attaches positional command line file inputs directly to [`join`](docs/language/operators/join.md) inputs (use [`file`](docs/language/operators/file.md) within a Zed program instead) (#4689)
* [Zeek](https://zeek.org/)-related docs have been moved to the Integrations area of the [Zed docs site](https://zed.brimdata.io/docs) (#4694, #4696)
* [`zed create`](docs/commands/super-db.md#create) now has a `-use` flag to set the newly-created pool as the default pool for future operations (#4656)
* Fix an issue where the [Zed Python client](docs/libraries/python.md) was incorrectly returning `False` for all `bool` values (#4706)
* Fix an issue where the `!=` operator was not returning correct results when comparing certain types (#4704)

## v1.8.1
* Send an HTTP 400 response instead of HTTP 500 for attempted deletes that find nothing to delete (#4618)
* Send an HTTP 400 response instead of HTTP 500 for queries that parse ok but fail to compile, such as searches lacking a leading [`from`](docs/language/operators/from.md) (#4620)
* Fix an issue where some regexps were not matching pool names when used in a `from` (#4619)

## v1.8.0
* Improve [`sort`](docs/language/operators/sort.md) performance for `duration` and `time` types (#4469)
* Improve performance and reduce memory used by `zed load` and `sort` on multi-GB inputs (#4476, #4484)
* Fix an issue where [meta-queries](docs/commands/super-db.md#meta-queries) were incorrectly returning results (#4474)
* The [`join` operator](docs/language/operators/join.md) now has an additional syntax that uses subqueries, which is more reminiscent of SQL (#4467, #4473, #4492, #4502)
* Improve performance when a Zed lake scan is not order sensitive (#4526)
* The [lake API documentation](docs/lake/api.md) now includes both request & response MIME types (#4512)
* Add more usage examples for the [`pass` operator](docs/language/operators/pass.md) in documentation (#4541)
* [`cast()`](docs/language/functions/cast.md) errors are now structured and contain more detail (#4548)
* A different Parquet library is now used, which allows for reading more dialects (#4547)
* Fix an issue where reading certain ZNG files via auto-detect caused a panic (#4569)
* Add a [`load` operator](docs/language/operators/load.md) that can be invoked in a Zed pipeline to commit data to a pool (#4471)
* Fix an issue where certain ZNG files could not be read and caused a `control` error (#4579)
* Fix an issue where `zed serve` would exit if it tried to write to a closed socket (#4587)
* Improve JSON output for Zed [maps](docs/formats/data-model.md#24-map) (#4589)
* Add the [`zed vacuum`](docs/commands/super-db.md#vacuum) command (#4577, #4598, #4600)

## v1.7.0
* Add [`regexp_replace()`](docs/language/functions/regexp_replace.md) function for replacing regular expression matches in a string (#4435, #4449)
* Add [documentation](docs/integrations/zed-lake-auth/index.md) showing how to configure Auth0 for authenticated access to a Zed lake service (#4439)
* Fix an issue where elements of map could not be accessed if the key was of a union type (#4447)
* Allow [`head`](docs/language/operators/head.md) operator to accept an expression (#4451)
* Allow [`tail`](docs/language/operators/tail.md) operator to accept an expression (#4464)

## v1.6.0

> ## Note: Changes to the Zed lake storage format
> 
> This release includes a change to the Zed lake storage format that is **not**
> backward compatible. To migrate your pools to the new format there is a 
> [Zed lake migration](https://github.com/brimdata/zed-lake-migration) kit and
> [specific guidance for users of the Zed CLI tools](https://github.com/brimdata/zed-lake-migration#zed-cli-tools).

* Zed lake storage format is now at version 3 (#4386, #4415)
* Allow loading and responses in [VNG](docs/formats/csup.md) format over the lake API (#4345)
* Fix an issue where [record spread expressions](docs/language/expressions.md#record-expressions) could cause a crash (#4359)
* Fix an issue where the Zed service `/version` endpoint returned "unknown" if it had been built via `go install` (#4371)
* Branch-level [meta-queries](docs/commands/super-db.md#meta-queries) on the `main` branch no longer require an explicit `@main` reference (#4377, #4394)
* Add `-defaultfmt` flag to `zed serve` to specify the lake API's default response format (#4379, #4396)
* Zed queries now appear in the lake log when `zed serve` is run at `-log.level debug` (#4385)
* Fix an issue where elements of complex [named types](docs/formats/data-model.md#3-named-type) could not be accessed (#4391)
* Add docs for the [`pass` operator](docs/language/operators/pass.md) and an example of [`join` on more than two inputs](docs/tutorials/join.md#joining-more-than-two-inputs) (#4403)
* When the result of [`aggregate`](docs/language/operators/aggregate.md) is a single value and there is no field name specified, the output is now that single value rather than a single-field record containing that value (#4420)

## v1.5.0
* Add `float16` primitive type (#4301)
* Add segment compression to the [VNG](docs/formats/csup.md) format (#4299)
* Add `-unbuffered` flag to `zed` and `zq` (#4320)
* Add `-csv.delim` flag to `zed` and `zq` for reading CSV with non-comma delimiter (#4325)
* Add `csv.delim` query parameter to lake API for reading CSV with non-comma delimiter (#4333)
* Add `-cors.origin` flag to `zed serve`, which defaults to a wildcard (#4334)
* Use `zng.` prefix for ZNG-related flags in `zed` and `zq` (#4336)
* Fix an issue where type decorators of union values were leaking into CSV output (#4338)

## v1.4.0
* The ZST format is now called [VNG](docs/formats/csup.md) (#4256)
* Allow loading of "line" format over the lake API (#4229)
* Allow loading of Parquet format over the lake API (#4235)
* Allow loading of Zeek TSV format over the lake API (#4246)
* Auto-detect Parquet and VNG formats (#4270)
* Add support for Arrow IPC stream format (#4252)
* Add pool level configuration to `zed manage` (#4234)
* The `mask` argument of the [`network_of()` function](docs/language/functions/network_of.md) now accepts an `ip`-type netmask instead of a `net` type (#4247, #4253)
* Drop support for the older "zng21" format (#4255)
* Add support for [user defined functions](docs/language/statements.md#func-statements) (UDFs) to the Zed language (#4264)
* Add journal snapshots, significantly improving performance when a pool contains many small commits (#4274)

## v1.3.0
* Optimize performance of filters on pool key fields (#3930)
* Improve handling of errors during [shaping](docs/language/shaping.md) (#4067, #4069)
* Allow use of a pool name regexp/glob pattern with the [`from` operator](docs/language/operators/from.md) (#4072, #4075)
* Add [`levenshtein()` function](docs/language/functions/levenshtein.md) for fuzzy string matching (#4104)
* Allow use of any filter with [`zed delete -where`](docs/commands/super-db.md#delete) (#4100, #4124, #4126, #4125, #4127)
* Add [`regexp()`](docs/language/functions/regexp.md) function for regular expression searches and capture groups (#4145, #4158)
* Add [`coalesce()`](docs/language/functions/coalesce.md) function for locating non-null/non-error values (#4172)
* Add `line` format for sourcing newline-delimited input as strings (#4175)
* Add [`collect_map()` aggregation function](docs/language/aggregates/collect_map.md) for constructing [maps](docs/formats/data-model.md#24-map) #4173

## v1.2.0
* Compress index values (#3974)
* Replace `-znglz4blocksize` flag with `-zngcompress` and `-zngframethresh` for `zed` and `zq` (#3967)
* Add `-splitsize` flag to `zed` and `zq` (#3957)
* Add `-aggmem` flag to `zed` and `zq` (#3921)
* Add `zed compact` (#3940)
* Remove `-repl` flag from `zc` (#3945)
* Always default to ZNG output for `-split` flag to `zed` and `zq` (#3938)
* Number `-split` flag files sequentially for `zed` and `zq`  (#3937)
* Remove index support for relative comparisons (#3932)
* Support `...` spread operator in [array expressions](docs/language/expressions.md#array-expressions) and [set expressions](docs/language/expressions.md#set-expressions) (#3910)
* [Python client](python/superdb): rework `zed.Client.query_raw()` to handle any format (#3911)
* Add support for Parquet API responses (#3909)
* Support the indexing operation on sets (#3908)
* Add `nullsMax` argument to [`compare() function`](docs/language/functions/compare.md) (#3898)

## v1.1.0

* Allow `-configdir`, `-lake`, `-q`, and `-use` flags before `zed` subcommands (#3852)
* Add [`assert` operator](docs/language/operators/assert.md) (#3858)
* Honor [`from` operator's](docs/language/operators/from.md) `format` option (#3845)
* Add [`compare()` function](docs/language/functions/compare.md) (#3832)
* Pretty-print ZSON type values (#3819)
* Allow conversion of time values to other numeric types (#3816)
* Remove scaling from duration and time conversions (#3809)
* Add [`over` expressions](docs/language/lateral-subqueries.md#lateral-expressions) (#3797)
* Add `-where` flag to [`zed delete`](docs/commands/super-db.md#delete) (#3791)
* Allow base62 object IDs in lake API request bodies (#3783)
* Remove `let` operator and [`over` operator](docs/language/operators/over.md)'s `as` clause (#3785)

## v1.0.0

* Comprehensive [documentation](docs/_index.md)
* Substantial improvements to the [Zed language](docs/language/_index.md)
* Revamped [`zed` command](docs/commands/super-db.md)
* New Zed lake format (see #3634 for a migration script)
* New version of the [ZNG format](docs/formats/bsup.md) (with read-only support for the previous version)
* New version of the [ZSON format](docs/formats/sup.md)

## v0.33.0

* `zapi`: Rename the `ZED_LAKE_HOST` environment variable to `ZED_LAKE` and rename the `-host` flag to `-lake` (#3280)
* `zq`: Improve ZNG read performance when the command line includes multiple input files (#3282)
* `zed lake serve`: Add the `-rootcontentfile` flag  (#3283)
* [Python client](python/superdb): Improve error messages (#3279)
* [Python client](python/superdb): Fix Zed `bytes` decoding (#3278)
* Detect CSV input (#3277)
* `zed lake serve`: Fix an issue where `POST /pool/{}/branch/{}` format detection errors caused a 500 response (#3272)
* Fix an issue where the ZSON parser failed to normalize maps and sets (#3273)
* [Python client](python/superdb): Add authentication (#3270)
* [Python client](python/superdb): Handle query errors  (#3269)
* Remove support for the TZNG format (#3263)
* `zapi`, `zed lake serve`: Add authentication with Auth0 (#3266)
* Fix an issue preventing casting from `ip` to `ip` (#3259)
* `zed lake serve`: Respect the Accept request header for `GET /events` (#3246)
* Add [function documentation](docs/language/functions/_index.md) (#3215)
* `zed lake serve`: Change the default response content encoding to ZSON (#3242)
* `zapi load`, `zed lake load`: Add the `-meta` flag to embed custom metadata in commits (#3237)

## v0.32.0

* Add `create_pool()` and `load()` methods to the [Python client](python) (#3232)
* Allow a leading `split` operator (#3230)
* Remove the `exists()` function in favor of `missing()` (#3225)
* Remove the `iso()` function in favor of `time()` (#3220)
* Remove deprecated `GET /pool` and `GET /pool/{pool}` from the Zed lake service API (#3219)
* Add bytes literals ("0x" followed by an even-length sequence of hexadecimal digits) to the Zed language (#3209)
* When sending a JSON response for `POST /query`, always send an array (#3207)
* Fix a panic when compiling `SELECT ... GROUP BY ...` (#3193)
* Fix a bug in which data loaded through the Zed lake service was stored uncompressed (#3198)
* Add all lake index commands to Zed lake service (#3181)
* Reorganize [language documentation](docs/language/_index.md) (#3187)
* Make `fuse()` output deterministic (#3190)
* Use lake indexes to speed up queries (#3158)
* Fix bug where constants blocked `from` operator wiring logic (#3185)
* Allow the dot operator to work on a union containing a record (#3178)
* Disable escaping of "&", "<", and ">" in JSON output (#3177)
* Change [`collect()`](docs/language/aggregates/collect.md) to handle heterogeneous types with a type union (#3176)
* Extend the [`join` operator](docs/language/operators/join.md) to support the `anti` join type (#3173)
* Make `lake index create` output the details of the newly created rule (#3168)
* Enable ANSI escapes in command output on Windows (#3164)
* Change `zed lake query -stats` output to ZSON (#3159)
* Fix a ZSON quoting bug for type value field names (#3154)
* Allow pool names (in addition to pool IDs) in Zed lake service API paths (#3144)

## v0.31.0

* Allow indexes to handle fields containing values of different types (#3141)
* Improve CSV writer performance (#3137)
* Fix an issue preventing use of a seek index containing nulls (#3138)
* Add `float32` primitive type (#3110)
* Add `len()` support for `bytes`, `error`, and map types (#3136)
* Allow empty ZSON maps (#3135)
* Fix an issue affecting `range` queries on a lake containing records with a missing or null pool key (#3134)
* Allow `from ( pass => ...; )` (#3133)
* Change Go marshaling struct field tag to `zed` from `zng` (#3130)
* Fix a panic when reading CSV containing an empty quoted field (#3128)
* Improve CSV output format (#3129)
* Detect JSON input containing a top-level array (#3124)
* Decode top-level JSON arrays incrementally (#3123)
* Remove PPL license (#3116)
* Change ZSON map syntax to `|{ key: value, ... }|` (#3111)
* Support revert for indexes (#3101)
* Rename `zson_parse()` to `parse_zson()` (#3092)
* Add `zed lake index update` and `zed api index update` commands (#3079, #3093)
* Add `parse_uri()` function (#3080, #3084)
* Add `from pool@branch:indexes` meta query (#3078)
* Fix an issue where `sort len(field)` produced incorrect output (#3045)
* Remove `POST /ast` and `POST /search` from the Zed lake service API (#3065)
* Fix an issue with with record aliases in `drop` (#3064)

## v0.30.0

As you can see below, there's been many changes since the last Zed GA release!  Highlights include:
* The introduction of Zed lakes for data storage, which include powerful
  Git-like branching. See the [Zed lake README](docs/commands/super-db.md)
  for details.
* Enhancements to the Zed language to unify search and expression syntax,
  introduce new operators and functions for data exploration and shaping, and
  more! Review the
  [Zed language docs](docs/language/_index.md)
  for details.

The exhaustive set of changes is listed below. Come talk to us on
[Slack](https://www.brimdata.io/join-slack/) if you have additional
questions.

---

* Revise Zed language to unify search and expression syntax (#2072, #2152, #2252, #2304, #2294)
* Add `join()` and `split()` functions for use on strings (#2098)
* Add array slice expressions (#2100)
* Fix an issue with connection resets after several minutes when posting data to S3 (#2106)
* Fix an issue with parsing IPv6 literals (#2112)
* Make the [`fuse`](docs/language/operators/fuse.md) operator work on nested records (#2052)
* Fix an issue where `cut(.)` could cause a `slice bounds out of range` panic (#2107)
* Add `is()`, `fields()`, and `exists()` functions (#2131)
* Add auto-detection of ZSON format (#2123)
* Fix an issue where [`cut`](docs/language/operators/cut.md) to the root would exit if the referenced field was missing from a record (#2121)
* Fix an issue where [`put`](docs/language/operators/put.md) to the root would panic on a non-record field (#2136)
* Add support for parsing map types in ZSON (#2142)
* Add a `fuse()` aggregate function (#2115)
* Remove backward compatibility with alpha ZNG format (#2158)
* Simplify ZSON by dropping type decorators when a complex value is fully implied (#2160)
* Add a `switch` operator to allow branched processing (#2087, #2364, #2318, #2336)
* Add constants and type literals to the Zed language (#2181)
* The `-I` option in `zq` is now used for file includes (and allows multiple files), while `-z` now used for compact ZSON output (#2180, #2208)
* Add support for shaping arrays and sets (#2173)
* Fix an issue where outer aliases were being lost when ZSON was read into ZNG (#2189)
* Add the `sample` operator that returns an example value for a named field, or for each unique record type (#2200, #2211, #2623)
* Make the current record (i.e., `this` or `.`) an implicit argument to `shape()` (#2199)
* Begin deprecating current TZNG format in favor of ZSON (#2208, #2312, #2333, #2338, #2337, #2339, #2340, #2355, #2367, #2377, #2387, #2388, #2389, #2395, #2477, #2485, #2480, #2513, #2520)
* Fix an issue where accidentally reading non-Zed binary data caused a `zq` panic (#2206)
* Fix an issue where time-sorted aggregations were returning non-deterministic results (#2220)
* Add canonical Zed and the `aggregate` operator as an explicit keyword before invoking aggregate functions (#2217, #2378, #2430, #2698)
* Add support for casting the `duration` type (#2194)
* Extend [`join`](docs/language/operators/join.md) to support `inner` (now the default), `left`, and `right` variations (#2210)
* Fix an issue where Zed would not compile on FreeBSD (#2233)
* Add the `zson_parse()` function (#2242)
* Fix an issue where filenames containing `:` could not be read (#2240)
* Handle aliases and typedefs in shaper functions, which also fixes a panic (#2257)
* Improve Zeek reader performance (#2265, #2268)
* Fix an issue where `const` references were not honored during query execution (#2260)
* Fix an issue where shapers did not handle aliases to different castable types (#2280)
* Add an `unflatten()` function that turns fields with dot-separated names into fields of nested records (#2277)
* Fix an issue where querying an index in a Zed lake did not return all matched records (#2273)
* Accept type definition names and aliases in shaper functions (#2289)
* Add a reference [shaper for Zeek data](docs/integrations/zeek/shaping-zeek-json.md) (#2300, #2368, #2448, #2489, #2601)
* Fix an issue where accessing a `null` array element in a `by` grouping caused a panic (#2310)
* Add support for parsing timestamps with offset format `±[hh][mm]` (#2297)
* Remove cropping from `shape()` (#2309)
* Apply a Zed shaper when reading Suricata EVE data, instead of legacy JSON typing (#2298, #2370, #2400)
* Add support for reading comma-separated value (CSV) files (#2317, #2858, #2942, #2963)
* Fix an issue where reading a Zeek TSV log line would cause a panic if it contained too few fields (#2325)
* Add a `shape` operator, which is useful for cleaning up CSV inputs (#2327)
* Fix an issue where querying a Zed lake index for a named field could cause a panic (#2319)
* Make casting to `time` and `duration` types more flexible (#2334, #2442)
* Fix an issue where `null` values were not output consistently in a grouped aggregation (#2363)
* Fix an issue where the confirmation messages from adding an index were sometimes incomplete (#2361)
* Finalize ZSON `duration` format to be an extension of [durations in Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations) (#2358, #2371, #2381, #2396, #2405)
* Add functions `missing()`, `has()`, and `nameof()` (#2393, #2708)
* Add prototype support for SQL expressions (#2392)
* Allow type definitions to be redefined (#2386)
* Fix an issue where casting to a named type caused the loss of the type definition name (#2384)
* Add support for Parquet output and rework the Parquet reader (#2227)
* Don't interpret the first `zq` argument as a query if there are no additional arguments (#2382)
* Fix an issue that was preventing the reference in an expression to a field name containing a `.` (#2407)
* Add support for ISO time literals and support durations and time literals in expressions (#2406)
* Add support for complex literals (#2403)
* Code/repo reorganization for phasing out "ZQL" or "Z" in favor of "Zed language", or just "Zed" if context allows (#2416, #2431, #2455, #2831)
* Support `in` with the `map` data type (#2421)
* Normalize map values created from Zed expressions (#2423)
* Switch to function-style casting (e.g., `int64(123)` instead of `123:int64`) (#2427, #2438)
* Allow shapers to to refer to the contents of input records to determine the type to apply (#2426)
* Fix an issue where referencing a non-existent table in a SQL query caused a panic (#2432)
* Accept `-` (stdin) as a `zapi` argument for loading data (#2435)
* Fix an issue where a single bad cast could cause input processing to halt (#2446)
* Create the `zed` command with sub-commands like `query` and `api`, but shortcut commands (e.g., `zq`, `zapi`) still remain (#2450, #2465, #2466, #2463, #2624, #2620)
* Rename `ZAR_ROOT` environment variable to `ZED_LAKE_ROOT` (#2469)
* Revise the top-level [Zed README](README.md) to reflect reorganization of the repo and new/changed tools (#2461)
* Remove the `-P` flag from `zq` in favor of using `from` in the Zed language (#2491)
* Add casting of the `net` data type (#2493, #2496)
* `zq` now reads its inputs sequentially rather than the prior merged behavior (#2492)
* Extend the `len()` function to return the number of fields in a record (#2494)
* Remove the `-E` flag in `zed` commands that displayed `time` values as epoch (#2495)
* Add the [Zed lake design](docs/commands/super-db.md) README document (#2500, #2569, #2595, #2781, #2940, #3014, #3034, #3035)
* Fix an issue where escaping quotes caused a parse error (#2510)
* Fix an issue where multiple ZSON type definitions would be output when only the first was needed (#2511)
* Use less buffer when decoding ZSON (#2515)
* Allow aliases of all primitive types to be expressed in ZSON (#2519)
* Revert the "auto-fuse CSV" behavior originally added in #1908 (#2522)
* Add support for Git-style Zed lakes (#2548, #2556, #2562, #2563, #2564, #2566, #2571, #2577, #2580, #2616, #2613, #2738, #2763, #2806, #2808, #2811, #2816, #2860, #2861, #2931, #2944, #2954, #2960, #2976, #2994, #3007, #3013, #3020, #3023, #3024, #3026, #3030, #3031, #3039, #3046)
* Add support for reading JSON format input data via `-i json` (#2573, #2608)
* Remove the legacy approach for applying Zed types to NDJSON input, as this is now done via Zed shapers (#2587)
* Fix a Go client issue where ZNG marshal of unexported struct fields caused a panic (#2589)
* Show a warning rather than failing when an unset value tries to be `cut` to the root (#2591)
* Standardize `-h` usage in Zed CLI tools for showing help text (#2596, #2618)
* Fix an issue where type names that started with primitive type names caused parse errors (#2612)
* Colorize `zson -Z` output (#2621)
* Remove pcap-related code, as this functionality has been moved to [Brimcap](https://github.com/brimdata/brimcap) (#2632)
* The role previously performed by `zqd` is now handled by `zed lake serve` (#2629, #2722)
* Revise ZJSON to encode types and type values using JSON structure instead of ZSON type strings (#2526)
* `this` can now be used to reference the current top-level record (formerly `.`, which may be deprecated in the future) (#2650)
* Rework dataflow model and Zed compiler optimizations (#2669)
* Add initial `explode` operator that can break values from complex fields out into separate records (#2673)
* Fix an issue where including a particular `time`-typed field in a shaper script caused errors with shaping other fields (#2685)
* Silently discard duplicate fields when reading NDJSON records, which works around Suricata bug 4016 (#2691)
* Fix an issue where ZSON type values were output without parentheses (#2700)
* Swallow single-backslash-escaped `/` when reading NDJSON, which allows for reading default Suricata EVE output (#2697)
* Improve the error message shown when no Zed lake root is specified (#2701, #2739)
* Require `on` in [`join`](docs/language/operators/join.md) syntax (#2698)
* Add a `typeunder()` function that returns the concrete type underlying a named type (#2709)
* Improve ZNG scanner performance via multi-threading (#2678, #2682)
* Fix an issue where a shaper created a corrupt `time`-typed value from an invalid timestamp rather than rejecting it (#2705)
* Simplify keyword search by requiring `:=` for assignment, `==` for comparison, and using `matches` for regex & glob match (#2692, #2744, #2773)
* Allow reading data from `http://` and `https://` targets (#2723, #2732)
* Support for arbitrary pool keys in Zed lakes (#2729, #2752)
* Add [API docs](docs/lake/api.md) for the Zed lake service (#2679)
* Support `from file` in Zed language in `zq`, which is particularly useful with [`join`](docs/language/operators/join.md) (#2753)
* Fix an issue where certain data could be queried successfully via `zq` but not if loaded into a Zed lake pool (#2755)
* Revise [Python client](python) docs to show double quotes during `pip` install, since Windows needs that (#2758)
* Fix an issue where a query was incorrectly parallelized by merging on the wrong key (#2760)
* Fix an issue where `len()` of a `null` array was evaluating to something greater than zero (#2761)
* Fix an issue where `sort` with no fields was ignoring alias types and nested fields when picking a sort field (#2762)
* Fix an issue where unexpected `cut: no record found` warnings were returned by `zed lake query` but not when the same data was queried via `zq` (#2764)
* Move and extend the [Zeek interoperability docs](docs/integrations/zeek/_index.md) (#2770, #2782, #2830)
* Create endpoints in the Zed lake service API that correspond to underlying Zed lake operations, and expose them via `zapi` commands (#2741, #2774, #2786, #2775, #2794, #2795, #2796, #2920, #2925, #2928)
* Fix an issue where `zq` would surface a syntax error when reading ZSON it had sent as output (#2792)
* Add an `/events` endpoint to the API, which can be used by clients such as the Brim app to be notified of pool updates (#2791)
* Simplify the ZSON `enum` type by removing the values from the list of symbols (#2820)
* Add Zed language documentation for the [`join` operator](docs/language/operators/join.md) (#2836)
* Fix an issue where reading ZNG input with more than 222 type definitions triggered a `zng type ID out of range` error (#2847)
* Have `put` only return the `a referenced field is missing` error on first occurrence (#2843)
* Fix an issue where a `zed lake query` triggered a `send on closed channel` panic (#2842)
* Allow casting to `bool` type (#2840)
* Fix an issue where `zq` would surface an error when reading ZST it had sent as output (#2854)
* Fix an issue where backend errors triggered by `zapi query` were not being surfaced (#2859)
* Have the [Python client](python) use the `/query` endpoint for the Zed lake (#2869)
* Minimize the amount of surrounding context shown when reporting parse errors (#2864)
* Field assignments in [`join`](docs/language/operators/join.md) now behave like [`cut`](docs/language/operators/cut.md) instead of `pick` (#2868)
* Add more background/context to Zed top-level language [README](docs/language/_index.md) (#2866 #2878, #2901)
* Unify `from`, `split`, and `switch` syntax to the forms shown [here](docs/language/_index.md) (#2871, #2896)
* Shapers can now cast values of the `null` type to any type (e.g., arrays or records) (#2882)
* Fix an issue where [`join`](docs/language/operators/join.md) was failing to match on values of comparable types (e.g., `string` and `bstring`) (#2880, #2884)
* Shapers can now cast a value to a `union` type (#2881)
* Introduce alternate `switch` syntax (#2888, #3004)
* When [`fuse`](docs/language/operators/fuse.md) encounters a field with the same name but different types, it now creates one field of `union` type rather than separate, uniquely-named fields (#2885, #2886)
* Fix an issue where [`fuse`](docs/language/operators/fuse.md) would consume too much memory when fusing many types (#2897, #2899)
* Emphasize in the [`sort`](docs/language/operators/sort.md) documentation that its output can be non-deterministic in the absence of an explicit field list (#2902)
* Remove the space separator before decorator in ZSON `-z` output (#2911)
* Fix an issue where handling of record aliases caused a failure to shape Zeek NDJSON data (#2904)
* Fix an issue where posting garbage input data to a pool caused an HTTP 500 response (#2924)
* Fix an issue where reading a ZNG file and outputting as CSV caused a deadlock (#2929)
* In a `from` clause, `range` is now used instead of `over` to specify a range scan over a data source (#2943)
* Fix a Zed language issue with parsing parenthesized search terms (#2951)
* Column headers in `-f table` outputs now reflect the case of the field name rather than always being uppercase (#2964)
* Reserved words in the Zed language can now be used in more places (e.g., field name references) without risk of collisions that would require escaping (#2968)
* Zed CLI tools now send human-readable ZSON by default if output is to a terminal, otherwise binary ZNG (#2979, #2985)
* Temporary directories for spill-to-disk operations now are prefixed with `zed-spill-` rather than `zq-spill-` (#2980)
* The [`put`](docs/language/operators/put.md) operator keyword is now optional (e.g., can write `x:=1` instead of `put x:=1`) (#2967, #2986, #3043)
* Fix an issue where a [`put`](docs/language/operators/put.md) on a nested record with an alias triggered a panic (#2990)
* Fix an issue where temporary spill-to-disk directories were not being deleted upon exit (#3009, #3010)
* Fix a ZSON issue with `union` types with alias decorators (#3015, #3016)
* The ZSON format has been changed such that integer type IDs are no longer output (#3017)
* Update the reference Zed shaper for Zeek ([docs](docs/integrations/zeek/shaping-zeek-json.md)) to reflect changes in Zeek release v4.1.0 (#3021)
* Fix an issue where backslash escapes in Zed regular expressions were not accepted (#3040)
* The ZST format has been updated to work for typedef'd outer records (#3047)
* Fix an issue where an empty string could not be output as a JSON field name (#3054)

## v0.29.0
* zqd: Update Zeek pointer to [v3.2.1-brim10](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim10) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#2081)
* zql: Add shaping primitive functions `cast()`, `fill()`, `crop()`, and `order()`, along with `fit()` and `shape()` (#1984, #2059, #2073, #2033)
* ZSON: Read ZSON incrementally rather than all at once (#2031)
* ZSON: Tighten whitespace in ZSON `-pretty=0` output (#2030)
* zql: Change parallel graph syntax to use `split` and `=>` (#2037)
* ZSON: Add `duration` to the implied type list (#2039)
* zq: Fix an issue with [`rename`](docs/language/operators/rename.md) where a subsequent `count()` would return no results (#2046)
* zq: Fix an issue where multiple alias typedefs were generated for the same type, causing a TZNG read failure (#2047)
* ZSON: Fix an issue with string scanning in the ZSON parser that caused the failure `parse error: parsing string literal` (#2048)
* zq: Fix an issue on Windows where `-` was not being treated as a way to read from stdin (#2061)
* zq: Add support in [`put`](docs/language/operators/put.md) for assigning to `.` and to nested fields (#2018)
* ZSON: Fix an issue where reading ZSON caused the failure `parse error: mismatched braces while parsing record type` (#2058)
* ZSON: Fix an issue where casting `null` values to string types caused invalid output (#2077)

## v0.28.0
**NOTE** - Beginning with this release, a subset of the source code in the
[github.com/brimdata/super](https://github.com/brimdata/super) GitHub repository is
covered by a source-available style license, the
[Polyform Perimeter License (PPL)](https://polyformproject.org/licenses/perimeter/1.0.0/).
We've moved the PPL-covered code under a `ppl/` directory in the repository.
The majority of our source code retains the existing BSD-3-Clause license.

The overwhelming majority of zq/zqd users and developers will not be impacted
by this change, including those using zq/zqd in commercial settings. The use of
the source-available Polyform Perimeter license prevents use cases like
marketing a work as a "as-a-service" style offering for server components like
zqd while using material covered under the PPL.

In general, we are making this change to ensure technology giants can't use the
PPL-covered code to make replacement offerings of our projects. We believe
users and developers should have access to the source code for our projects,
and we need a sustainable business model to continue funding our work. Using
the source-available Polyform Perimeter license on portions of the source code
lets us realize both.

For more detail regarding licensing, see the
[CONTRIBUTING.md](CONTRIBUTING.md)
doc, and feel free to come talk to us on
[Slack](https://www.brimdata.io/join-slack/) if you have additional
questions.

---

* zqd: Update Zeek pointer to [v3.2.1-brim9](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim9) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#2010)
* zqd: Update Suricata pointer to [v5.0.3-brim1](https://github.com/brimdata/build-suricata/releases/tag/v5.0.3-brim1) which disables checksum checks, allowing for alert creation on more types of pcaps (#1975)
* ZSON: Update [Zeek Interoperability doc](docs/integrations/zeek/data-type-compatibility.md) to include current ZSON syntax (#1956)
* zq: Ensure the output from the [`fuse`](docs/language/operators/fuse.md) operator is deterministic (#1958)
* zq: Fix an issue where the presence of the Greek µ character caused a ZSON read parsing error (#1967)
* zqd: Fix an issue where Zeek events generated during pcap import and written to an archivestore were only visible after ingest completion (#1973)
* zqd: Change the logger configuration to output stacktraces on messages of level "warn" and higher (#1990)
* zq: Update [performance results](performance/README.md) to include ZSON read/write (#1974)

## v0.27.1
* zq: Fix an issue where nested nulls caused a panic in CSV output (#1954)

## v0.27.0
* zqd: Update Zeek pointer to [v3.2.1-brim8](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim8) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#1928)
* ZSON: Allow characters `.` and `/` in ZSON type names, and fix an issue when accessing fields in aliased records (#1850)
* ZSON: Add a ZSON marshaler and clean up the ZNG marshaler (#1854)
* zq: Add the `source` field to the JSON typing config to prepare for Zeek v4.x `weird` events (#1884)
* zq: Add initial Z "shaper" for performing ETL on logs at import time (#1870)
* zq: Make all aggregators decomposable (#1893)
* zq/zqd: Invoke [`fuse`](docs/language/operators/fuse.md) automatically when CSV output is requested (#1908)
* zq: Fix an issue where [`fuse`](docs/language/operators/fuse.md) was not preserving record order (#1909)
* zar: Create indices when data is imported or chunks are compacted (#1794)
* zqd: Fix an issue where warnings returned from the `/log/path` endpoint were being dropped (#1903)
* zq: Fix an issue where an attempted search of an empty record caused a panic (#1911)
* zq: Fix an issue where a top-level field in a Zeek TSV log was incorrectly read into a nested record (#1930)
* zq: Fix an issue where files could not be opened from Windows UNC paths (#1929)

## v0.26.0
* zqd: Update Zeek pointer to [v3.2.1-brim7](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim7) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#1855)
* zq: Improve the error message shown when row size exceeds max read buffer (#1808)
* zqd: Remove `listen -pprof` flag (profiling data is now always made available) (#1800)
* ZSON: Add initial ZSON parser and reader (#1806, #1829, #1830, #1832)
* zar: Use a newly-created index package to create archive indices (#1745)
* zq: Fix issues with incorrectly-formatted CSV output (#1828, #1818, #1827)
* zq: Add support for inferring data types of "extra" fields in imported NDJSON (#1842)
* zqd: Send a warning when unknown fields are encountered in NDJSON logs generated from pcap ingest (i.e. Suricata) (#1847)
* zq: Add NDJSON typing configuration for the Suricata "vlan" field (#1851)

## v0.25.0
* zqd: Update Zeek pointer to [v3.2.1-brim6](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim6) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#1795)
* zqd: Update Suricata pointer to [v5.0.3-brimpre2](https://github.com/brimdata/build-suricata/releases/tag/v5.0.3-brimpre2) to generate alerts for imported pcaps (#1729)
* zqd: Make some columns more prominent (moved leftward) in Suricata alert records (#1749)
* zq: Fix an issue where returned errors could cause a panic due to type mismatches (#1720, #1727, #1728, #1740, #1773)
* python: Fix an issue where the [Python client](https://medium.com/brim-securitys-knowledge-funnel/visualizing-ip-traffic-with-brim-zeek-and-networkx-3844a4c25a2f) did not generate an error when `zqd` was absent (#1711)
* zql: Allow the `len()` function to work on `ip` and `net` types (#1725)
* ZSON: Add a [draft specification](docs/formats/sup.md) of the new ZSON format (#1715, #1735, #1741, #1765)
* zng: Add support for marshaling of `time` values (#1743)
* zar: Fix an issue where a `couldn't read trailer` failure was observed during a `zar zq` query (#1748)
* zar: Fix an issue where `zar import` of a 14 GB data set triggered a SEGV (#1766)
* zql: Add a new [`drop`](docs/language/operators/drop.md) operator, which replaces `cut -c` (#1773)
* zql: Add a new `pick` operator, which acts like a stricter [`cut`](docs/language/operators/cut.md) (#1773, #1788)
* zqd: Improve performance when listing Spaces via the API (#1779, #1786)

## v0.24.0
* zq: Update Zeek pointer to [v3.2.1-brim5](https://github.com/brimdata/zeek/releases/tag/v3.2.1-brim5) which provides the latest [geolocation](https://github.com/brimdata/brim/wiki/Geolocation) data (#1713)
* zql: For functions, introduce "snake case" names and deprecate package syntax (#1575, #1609)
* zql: Add a `cut()` function (#1585)
* zar: Allow `zar import` of multiple paths (#1582)
* zar: Fix an issue where a bare word `zar zq` search could cause a panic (#1590)
* zq: Update Go dependency to 1.15 (#1547)
* zar: Fix an issue where `zar zq` yielded incorrect event counts compared to plain `zq` (#1588, #1602)
* zq: Fix a memory bug in `collect()` that caused incorrect results (#1598)
* zqd: Support log imports over the network (#1336)
* zq: Update [performance results](performance/README.md) to reflect recent improvements (#1605, #1669, #1671)
* zq: Move Zeek & Suricata dependencies into `package.json` so Brim can point to them also (#1607, #1610)
* zql: Add support for [aggregation-less grouping](docs/language/operators/aggregate.md) (#1615, #1623)
* zqd: Run `suricata-update` at startup when Suricata pcap analysis is enabled (#1586)
* zqd: Add example Prometheus metrics (#1627)
* zq: Fix an issue where doing `put` of a null value caused a crash (#1631)
* zq: Add `-P` flag to connect two or more inputs to a ZQL query that begins with a parallel flow graph (#1628, #1618)
* zql: Add an initial `join` operator (#1632, #1642)
* zar: Fix an issue where consecutive timestamps caused seek index misses (#1634)
* zar: Fix an issue where time grouping was not working correctly for zar archives (#1650)
* zq/zql: Add support for ZQL comments, multi-line queries, and a `-z` flag for reading ZQL from a file (#1654)
* zqd: Automatically compact data via a background task (#1625)
* zq: Make ordered merge deterministic (#1663)
* zq: Fix a performance regression (#1672)
* zq: Fix an issue where the JavaScript and Go versions of ASTs could differ (#1665)
* zq: Fix an issue where a lone hyphen in an NDJSON value was output incorrectly (#1673)
* zq: Add an experimental writer for a new format called ZSON (#1681)
* zar: Fix an issue during import that could buffer too much data (#1652, #1696)
* zql: Add a `network_of()` function for mapping IP addresses to CIDR nets (#1700)
* zql: Add a [docs example](docs/language/operators/aggregate.md) showing `by` grouping with non-present fields (#1703)

## v0.23.0
* zql: Add `week` as a unit for [time grouping with `every`](docs/language/functions/every.md) (#1374)
* zq: Fix an issue where a `null` value in a [JSON type definition](docs/integrations/zeek/_index.md) caused a failure without an error message (#1377)
* zq: Add [`zst` format](docs/formats/csup.md) to `-i` and `-f` command-line help (#1384)
* zq: ZNG spec and `zq` updates to introduce the beta ZNG storage format (#1375, #1415, #1394, #1457, #1512, #1523, #1529), also addressing the following:
   * New data type `bytes` for storing sequences of bytes encoded as base64 (#1315)
   * Improvements to the `enum` data type (#1314)
   * Special characters like `.` and `@` may now appear in field names (#1291)
   * A `set` may now only support elements of a single type (#1220, #1515)
   * Remove the `byte` type from the spec in favor of `uint8` (#1316)
   * New data type `map`, which is like `set` but the contents are key value pairs where only keys need to be unique and the canonical order is based on the key order (#1317)
   * First-class ZNG types (#1365)
   * New numeric data types `float16` and `float32` (not yet implemented in `zq`) (#1312, #1514)
   * New numeric data type `decimal` (not yet implemented in `zq`) (#1522)
* zq: Add backward compatibility for reading the alpha ZNG storage format (#1386, #1392, #1393, #1441)
* zqd: Check and convert alpha ZNG filestores to beta ZNG (#1574, #1576)
* zq: Fix an issue where spill-to-disk file names could collide (#1391)
* zq: Allow the [`fuse` operator](docs/language/operators/fuse.md) to spill-to-disk to avoid memory limitations (#1355, #1402)
* zq: No longer require `_path` as a first column in a [JSON type definition](docs/integrations/zeek/_index.md) (#1370)
* zql: Improve ZQL docs for [aggregate functions](docs/language/operators/aggregate.md) and grouping (#1385)
* zql: Point links for developer docs at [pkg.go.dev](https://pkg.go.dev/) instead of [godoc.org](https://godoc.org/) (#1401)
* zq: Add support for timestamps with signed timezone offsets (#1389)
* zq: Add a [JSON type definition](docs/integrations/zeek/_index.md) for alert events in [Suricata EVE logs](https://suricata.readthedocs.io/en/suricata-5.0.2/output/eve/eve-json-output.html) (#1400)
* zq: Update the [ZNG over JSON (ZJSON)](docs/formats/zjson.md) spec and implementation (#1299)
* zar: Use buffered streaming for archive import (#1397)
* zq: Add an `ast` command that prints parsed ZQL as its underlying JSON object (#1416)
* zar: Fix an issue where `zar` would SEGV when attempting to query a non-existent index (#1449)
* zql: Allow sort by expressions and make `put`/`cut` expressions more flexible (#1468)
* zar: Move where chunk metadata is stored (#1461, #1528, #1539)
* zar: Adjust the `-ranges` option on `zar ls` and `zar rm` (#1472)
* zq: Choose default memory limits for `sort` & `fuse` based on the amount of system memory (#1413)
* zapi: Fix an issue where `create` and `find` were erroneously registered as root-level commands (#1477)
* zqd: Support pcap ingest into archive Spaces (#1450)
* zql: Add [`where` filtering](docs/language/operators/aggregate.md) for use with aggregate functions (#1490, #1481, #1533)
* zql: Add [`union()`](docs/language/aggregates/union.md) aggregate function (#1493, #1534)
* zql: Add [`collect()`](docs/language/aggregates/collect.md) aggregate function (#1496, #1534)
* zql: Add [`and()`](docs/language/aggregates/and.md) and [`or()`](docs/language/aggregates/or.md) aggregate functions (#1497, #1534)
* zq: Fix an issue where searches did not match field names of records with unset values (#1511)
* zq: Fix an issue where searches were not reaching into records inside arrays (#1516)
* zar: Support microindexes created with a sorted flow of records in descending order (#1526)
* zapi: Allow `zapi post` of S3 objects (#1532)
* zar: Add the `zar compact` command for combining overlapping chunk files into single chunks (#1531)
* zar: Use chunk seek index for searching chunk data files (#1537)
* zq: Make timestamp output formatting consistent (#1550, #1551, #1557)
* zq: Update LZ4 dependency to improve performance (#1556)
* zq: Fix an issue where TZNG fields containing `]` were treated as a syntax error (#1561)
* zar: Fix an issue where the `zar import` target size didn't take compression into account (#1565)
* zapi: Add a `-stats` option to `zapi pcappost` (#1538)
* zqd: Add a Python `zqd` API client for use with tools like [JupyterLab](https://jupyterlab.readthedocs.io/en/stable/) (#1564)

## v0.22.0
* zq: Change the implementation of the `union` type to conform with the [ZNG spec](docs/formats/bsup.md#215-union-typedef) (#1245)
* zq: Make options/flags and version reporting consistent across CLI tools (#1249, #1254, #1256, #1296, #1323, #1334, #1328)
* zqd: Fix an issue that was preventing flows in nanosecond pcaps from opening in Brim (#1243, #1241)
* zq: Fix an issue where the TZNG reader did not recognize a bad record type as a syntax error (#1260)
* zq: Add a CSV writer (`-f csv`) (#1267, #1300)
* zqd: Add an endpoint for returning results in CSV format (#1280)
* zqd: Add an endpoint for returning results in NDJSON format (#1283)
* zapi: Add an option to return results as a JSON array (`-e json`) (#1285)
* zapi: Add output format options/flags to `zapi get` (#1278)
* zqd: Add an endpoint for creating/querying search indexes (#1272)
* zapi: Add commands `zapi index create|find` for creating/querying search indexes (#1289)
* pcap: Mention ICMP protocol filtering (`-p icmp`) in help text (#1281)
* zq: Point to new Slack community URL https://www.brimsecurity.com/join-slack/ in docs (#1304)
* zqd: Fix an issue where starting `zqd listen` created excess error messages when subdirectories were present (#1303)
* zql: Add the [`fuse` operator](docs/language/operators/fuse.md) for unifying records under a single schema (#1310, #1319, #1324)
* zql: Fix broken links in documentation (#1321, #1339)
* zst: Introduce the [ZST format](docs/formats/csup.md) for columnar data based on ZNG (#1268, #1338)
* pcap: Fix an issue where certain pcapng files could fail import with a `bad option length` error (#1341)
* zql: Document the `**` operator for type-specific searches that look within nested records (#1337)
* zar: Change the archive data file layout to prepare for handing chunk files with overlapping ranges and improved S3 support (#1330)
* zar: Support archive data files with overlapping time spans (#1348)
* zqd: Add a page containing guidance for users that directly access the root `zqd` endpoint in a browser (#1350)
* pcap: Add a `pcap info` command to print summary/debug details about a packet capture file (#1354)
* zqd: Fix an issue with empty records (#1353)
* zq: Fix an issue where interrupted aggregations could leave behind temporary files (#1357)
* zng: Add a marshaler to generate ZNG streams from native Go values (#1327)

## v0.21.0
* zq: Improve performance by making fewer API calls in S3 reader (#1191)
* zq: Use memory more efficiently by reducing allocations (#1190, #1201)
* zqd: Fix an issue where a pcap moved/deleted after import caused a 404 response and white screen in Brim (#1198)
* zqd: Include details on adding observability to the docs for running `zqd` in Kubernetes (#1173)
* zq: Improve performance by removing unnecessary type checks (#1192, #1205)
* zq: Add additional Boyer-Moore optimizations to improve search performance (#1188)
* zq: Fix an issue where data import would sometimes fail with a "too many files" error (#1210)
* zq: Fix an issue where error messages sometimes incorrectly contained the text "(MISSING)" (#1199)
* zq: Fix an issue where non-adjacent record fields in Zeek TSV logs could not be read (#1225, #1218)
* zql: Fix an issue where `cut -c` sometimes returned a "bad uvarint" error (#1227)
* zq: Add support for empty ZNG records and empty NDJSON objects (#1228)
* zng: Fix the tag value examples in the [ZNG spec](docs/formats/bsup.md) (#1230)
* zq: Update LZ4 dependency to eliminate some memory allocations (#1232)
* zar: Add a `-sortmem` flag to allow `zar import` to use more memory to improve performance (#1203)
* zqd: Fix an issue where file paths containing URI escape codes could not be opened in Brim (#1238)

## v0.20.0
* zqd: Publish initial docs for running `zqd` in Kubernetes (#1101)
* zq: Provide a better error message when an invalid IP address is parsed (#1106)
* zar: Use single files for microindexes (#1110)
* zar: Fix an issue where `zar index` could not handle more than 5 "levels" (#1119)
* zqd: Fix an issue where `zapi pcappost` incorrectly reported a canceled operation as a Zeek exit (#1139)
* zar: Add support for empty microindexes, also fixing an issue where `zar index` left behind empty files after an error (#1136)
* zar: Add `zar map` to handle "for each file" operations (#1138, #1148)
* zq: Add Boyer-Moore filter optimization to ZNG scanner to improve performance (#1080)
* zar: Change "zdx" to "microindex" (#1150)
* zar: Update the `zar` README to reflect recent changes in commands/output (#1149)
* zqd: Fix an issue where text stack traces could leak into ZJSON response streams (#1166)
* zq: Fix an issue where an error "slice bounds out of range" would be triggered during attempted type conversion (#1158)
* pcap: Fix an issue with pcapng files that have extra bytes at end-of-file (#1178)
* zqd: Add a hidden `-brimfd` flag to `zqd listen` so that `zqd` can close gracefully if Brim is terminated abruptly (#1184)
* zar: Perform `zar zq` queries concurrently where possible (#1165, #1145, #1138, #1074)

## v0.19.1

* zq: Move third party license texts in this repository to a single [acknowledgments.txt](acknowledgments.txt) file (#1107)
* zq: Automatically load AWS config from shared config file `~/.aws/config` by default (#1109)
* zqd: Fix an issue with excess characters in Space names after upgrade (#1112)

## v0.19.0
* zq: ZNG output is now LZ4-compressed by default (#1050, #1064, #1063, [ZNG spec](docs/formats/bsup.md#2-the-super-binary-format))
* zar: Adjust import size threshold to account for compression (#1082)
* zqd: Support starting `zqd` with datapath set to an S3 path (#1072)
* zq: Fix an issue with panics during pcap import (#1090)
* zq: Fix an issue where spilled records were not cleaned up if `zq` was interrupted (#1093, #1099)
* zqd: Add `-loglevel` flag (#1088)
* zq: Update help text for `zar` commands to mention S3, and other improvements (#1094)
* pcap: Fix an out-of-memory issue during import of very large pcaps (#1096)

## v0.18.0
* zql: Fix an issue where data type casting was not working in Brim (#1008)
* zql: Add a new [`rename` operator](docs/language/operators/rename.md) to rename fields in a record (#998, #1038)
* zqd: Fix an issue where API responses were being blocked in Brim due to commas in Content-Disposition headers (#1014)
* zq: Improve error messaging on S3 object-not-found (#1019)
* zapi: Fix an issue where `pcappost` run with `-f` and an existing Space name caused a panic (#1042)
* zqd: Add a `-prometheus` option to add [Prometheus](https://prometheus.io/) metrics routes the API (#1046)
* zq: Update [README](README.md) and add docs for more command-line tools (#1049)

## v0.17.0
* zq: Fix an issue where the inferred JSON reader crashed on multiple nested fields (#948)
* zq: Introduce spill-to-disk for very large grouped aggregations (#932, #963)
* zql: Use syntax `c=count()` instead of `count() as c` for naming the field that holds the value returned by an aggregate function (#950)
* zql: Fix an issue where attempts to `tail` too much caused a panic (#958)
* zng: Readability improvements in the [ZNG specification](docs/formats/bsup.md) (#935)
* zql: Fix an issue where use of `cut`, `put`, and `cut` in the same pipeline caused a panic (#980)
* zql: Fix an issue that was preventing the `uniq` operator from  working in the Brim app (#984)
* zq: Fix an issue where spurious type IDs were being created (#964)
* zql: Support renaming a field via the `cut` operator (#969)

## v0.16.0
* zng: Readability improvements in the [ZNG specification](docs/formats/bsup.md) (#897, #910, #917)
* zq: Support directory output to S3 (#898)
* zql: Grouped aggregation no longer emits records in "deterministic but undefined" order (#914)
* zqd: Revise constraints on Space names (#853, #926, #944, #945)
* zqd: Fix an issue where a file replacement race could cause an "access is denied" error in Brim during pcap import (#925)
* zng: Revise [Zeek compatibility](docs/integrations/zeek/data-type-compatibility.md) doc (#919)
* zql: Clarify [`cut` operator documentation](docs/language/operators/cut.md) (#924)
* zqd: Fix an issue where an invalid 1970 Space start time could be created in Brim during pcap import (#938)

## v0.15.0
* pcap: Report more detailed error information (#844)
* zql: Add a new function `Time.trunc()` (#842)
* zql: Support grouping by computed keys (#860)
* zq: Change implementation of `every X` to use a computed grouping key (#893)
* zql: Clean up the [ZQL docs](docs/language/_index.md) (#884)
* zql: Change `cut` operator to emit any matching fields (#899)
* zq: Allow output to an S3 bucket (#889)

## v0.14.0
* zq: Add support for reading from S3 buckets (#733, #780, #783)
* zq: Add initial support for reading Parquet files (only via `-i parquet`, no auto-detection) (#736, #754, #774, #780, #782, #820, #813, #830, #825, #834)
* zq: Fix an issue with reading/writing recursively-nested NDJSON events (#748)
* zqd: Begin using a "runner" to invoke Zeek for processing imported pcaps (#718, #788)
* zq: Fix issues related to reading NDJSON during format detection (#752)
* zqd: Include stack traces on panic errors (#732)
* zq: Handle `\r\n` line endings generated by MinGW (Windows) Zeek (#775)
* zq: Support scientific notation for integer types (#768)
* zql: Add cast syntax to expressions (#765, #784)
* zq: Fix an issue where reads from stdin were described as being from `-` (#777)
* zq: Improve an NDJSON parsing error to be more detailed than "bad format" (#776)
* zjson: Fix an issue with aliases in the zjson writer (#793)
* zq: Fix an issue where typed JSON reads could panic when a field that was expected to contain an array instead contained a scalar (#799)
* zq: Fix an issue with ZNG handling of aliases on records (#801)
* zq: Fix an issue with subnet searches (#807)
* zapi: Introduce `zapi`, a simple CLI for interacting with `zqd` servers (#802, #809, #812)
* zq: Add arguments to generate CPU/memory profiles (#814)
* zql: Introduce time conversion functions (#822)
* zq: Ensure Spaces have non-blank names (#826)

## v0.13.1
* zq: Fix an issue with stream reset that was preventing the pcap button in Brim from activating (#725)
* zql: Allow multiple fields to be written from `put` operator (#697)

## v0.13.0
* zqd: Enable time indexing to provide faster query response in narrower time ranges (#647)
* zql: Make ipv4 subnet bases contain 4 octets to remove ambiguity between fractions & CIDR (#670)
* zq: Use an external sort for large inputs (removes the 10-million line `sort` limit) (#527)
* zq: Fix an issue where duplicate field names could be produced by grouped aggregation (#676)
* zar: Introduce an experimental prototype for working with archived logs (#700)
* zq: Support recursive record nesting in Zeek reader/writer (#715)
* zqd: Zeek log import support needed for Brim (#616, #517, #608, #592, #592, #582, #709)

## v0.12.0
* zql: Introduce `=~` and `!~` operators in filters for globs, regexps, and matching addresses against subnets (#604, #620)
* zq: When input auto-detect fails, include each attempted format's error (#616)
* zng: Binary format is now called "ZNG" and text format is called "TZNG" ("BZNG" has been retired) (#621, #630, #656)
* zql: `cut` now has a `-c` option to show all fields _not_ in the provided list (#639, #655)
* zq: Make `-f zng` (binary ZNG) the default `zq` output format, and introduce `-t` as shorthand for `-f tzng` (#654)

## v0.11.1
* zqd: Send HTTP status 200 for successful pcap search (#605)

## v0.11.0
* zql: Improve string search matching on field names (#570)
* pcap: Better handling of empty results (#572)
* zq: Introduce `-e` flag to allow for continued reads during input errors (#577)
* pcap: Allow reading of pcap files that have a capture length that exceeds the original length of the packet (#584)
* zqd: Fix an issue that was causing the histogram to draw incorrectly in Brim app (#602)

## v0.10.0

* zql: Let text searches match field names as well as values (#529)
* zql: Fix an issue where ZQL queries exceeding 255 chars caused a crash (#543)
* zql: Make searches case-insensitive by default (#536)
* Fix an issue where the Zeek reader failed to read whitespace from the rightmost column (#552)

## v0.9.0

* zql: Emit warnings from `put` operator (#477)
* zql: Add string functions (#475)
* zql: Narrow the use of `len()` to only sets/vectors, introduce new functions for string length (#485)
* zql: Add ternary conditional operator (#484)
* zqd: Add waterfall logger (#492)
* zqd: Make http shutdown more graceful (#500)
* zqd: Make space deletion cancel and await other operations (#451)

## v0.8.0

* zql: add the `put` operator that adds or updates fields using a computed
  expression. (#437)
* zql: add functions for use with put, like `Math.min`, `Math.max`, and others.
  (#453, #459, #461, #472)
* zq: support reading ndjson with user supplied type information. (#441)
* Fix an issue reading pcaps with snaplen=0. (#462)

## v0.7.0

* Address ingest issues for packet captures in legacy pcap format.
* Calculate and respond with packet capture time range at the start of ingest,
  so that Brim can immediately display the space's time range.

## v0.6.0

* zq now displays warnings by default; the "-W" flag is removed, replaced by
  the "-q" for quieting warnings.
* Update license to reflect new corporate name.
* Address ingest issues for some pcapng packet captures.
* Address ingest issues for file or path names that require uri encoding.

## v0.5.0

* Support search queries during pcap ingestion.
* Improved error reporting in zqd, especially during pcap ingestion.
* Improved performance of space info api.
* zqd supports ingesting pcapng formatted packet capture files.

## v0.4.0

* zqd adds an endpoint to create a new empty space via post
* zqd adds an endpoint to post packet captures that are indexed and turned into Zeek logs

## v0.3.0

* zqd adds -datadir flag for space root directory.
* zqd adds -version flag.
* Add pcap command to interact with packet capture files.

## v0.2.0

* Per-platform binaries will be available as Github release assets.
* zql examples under zql/docs are now verified via `make test-heavy`.
* Negative integers and floats are accepted in zql expressions.
* Internal integer types now match the ZNG specification.
* Fixed comparisons of aliased types.

## v0.1.0

* zq moves from github.com/mccanne/zq to github.com/brimdata/zed.
* Parser and AST moved to this repository from github.com/looky-cloud/lookytalk.
* Query language name changed to ZQL.
* ZNG specification added.

## v0.0.1

* Initial release of zq.
