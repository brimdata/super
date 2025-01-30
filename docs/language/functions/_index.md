---
weight: 2
title: Functions
---

Functions appear in [expression](../expressions.md) context and
take Zed values as arguments and produce a value as a result. In addition to
the built-in functions listed below, Zed also allows for the creation of
[user-defined functions](../statements.md#func-statements).

A function-style syntax is also available for converting values to each of
Zed's [primitive types](../../formats/zed.md#1-primitive-types), e.g.,
`uint8()`, `time()`, etc. For details and examples, read about the
[`cast` function](cast.md) and how it is [used in expressions](../expressions.md#casts).

* [abs](abs.md) - absolute value of a number
* [base64](base64.md) - encode/decode base64 strings
* [bucket](bucket.md) - quantize a time or duration value into buckets of equal widths
* [cast](cast.md) - coerce a value to a different type
* [ceil](ceil.md) - ceiling of a number
* [cidr_match](cidr_match.md) - test if IP is in a network
* [compare](compare.md) - return an int comparing two values
* [coalesce](coalesce.md) - return first value that is not null, a "missing" error, or a "quiet" error
* [crop](crop.md) - remove fields from a value that are missing in a specified type
* [error](error.md) - wrap a value as an error
* [every](every.md) - bucket `ts` using a duration
* [fields](fields.md) - return the flattened path names of a record
* [fill](fill.md) - add null values for missing record fields
* [flatten](flatten.md) - transform a record into a flattened map
* [floor](floor.md) - floor of a number
* [grep](grep.md) - search strings inside of values
* [grok](grok.md) - parse a string into a structured record
* [has](has.md) - test existence of values
* [hex](hex.md) - encode/decode hexadecimal strings
* [has_error](has_error.md) - test if a value has an error
* [is](is.md) - test a value's type
* [is_error](is_error.md) - test if a value is an error
* [join](join.md) - concatenate array of strings with a separator
* [kind](kind.md) - return a value's type category
* [ksuid](ksuid.md) - encode/decode KSUID-style unique identifiers
* [len](len.md) - the type-dependent length of a value
* [levenshtein](levenshtein.md) Levenshtein distance
* [log](log.md) - natural logarithm
* [lower](lower.md) - convert a string to lower case
* [map](map.md) - apply a function to each element of an array or set
* [missing](missing.md) - test for the "missing" error
* [nameof](nameof.md) - the name of a named type
* [nest_dotted](nest_dotted.md) - transform fields in a record with dotted names to nested records
* [network_of](network_of.md) - the network of an IP
* [now](now.md) - the current time
* [order](order.md) - reorder record fields
* [parse_uri](parse_uri.md) - parse a string URI into a structured record
* [parse_zson](parse_zson.md) - parse ZSON text into a Zed value
* [pow](pow.md) - exponential function of any base
* [quiet](quiet.md) - quiet "missing" errors
* [regexp](regexp.md) - perform a regular expression search on a string
* [regexp_replace](regexp_replace.md) - replace regular expression matches in a string
* [replace](replace.md) - replace one string for another
* [round](round.md) - round a number
* [rune_len](rune_len.md) - length of a string in Unicode code points
* [shape](shape.md) - apply cast, fill, and order
* [split](split.md) - slice a string into an array of strings
* [sqrt](sqrt.md) - square root of a number
* [strftime](strftime.md) - format time values
* [trim](trim.md) - strip leading and trailing whitespace
* [typename](typename.md) - look up and return a named type
* [typeof](typeof.md) - the type of a value
* [typeunder](typeunder.md) - the underlying type of a value
* [under](under.md) - the underlying value
* [unflatten](unflatten.md) - transform a record with dotted names to a nested record
* [upper](upper.md) - convert a string to upper case
