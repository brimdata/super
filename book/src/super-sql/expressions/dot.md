## Dot

Record fields are dereferenced with the dot operator `.` as is customary
in other languages and have the form
```
<value> . <id>
```
where `<id>` is an identifier representing the field name referenced.
If a field name is not representable as an identifier, then [indexing](#indexing)
may be used with a quoted string to represent any valid field name.
Such field names can be accessed using
[`this`](pipeline-model.md#the-special-value-this) and an array-style reference, e.g.,
`this["field with spaces"]`.

XXX Backtick-escaped identifier

If the dot operator is applied to a value that is not a record
or if the record does not have the given field, then the result is
`error("missing")`.