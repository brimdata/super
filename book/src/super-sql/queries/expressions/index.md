### Indexing

The index operation is denoted with square brackets and can be applied to
various data types having the form:
```
<value> [ <index> ]
```
If the `<value>` expression is a record, then the `<index>` operand
must be coercible to a string and the result is the record's field
of that name.

If the `<value>` expression is an array, then the `<index>` operand
must be coercible to an integer and the result is the
value in the array of that index.

If the `<value>` expression is a set, then the `<index>` operand
must be coercible to an integer and the result is the
value in the set of that index ordered by total order of values.

If the `<value>` expression is a map, then the `<index>` operand
is presumed to be a key and the corresponding value for that key is
the result of the operation.  If no such key exists in the map, then
the result is `error("missing")`.

If the `<value>` expression is a string, then the `<index>` operand
must be coercible to an integer and the result is an integer representing
the unicode code point at that offset in the string.

If the `<value>` expression is type `bytes`, then the `<index>` operand
must be coercible to an integer and the result is an unsigned 8-bit integer
representing the byte value at that offset in the bytes sequence.
