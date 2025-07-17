# Super-structured Formats

This section contains the [data model definition](model.md) for super-structured data 
along with a set of concrete formats that all implement this same data model,
providing a unified approach to row, columnar, and human-readable formats:

* [Super (SUP)](sup.md) is a human-readable format for super-structured data.  All JSON
documents are SUP values as the SUP format is a strict superset of the JSON syntax.
* [Super Binary (BSUP)](bsup.md) is a row-based, binary representation somewhat like
[Avro](https://avro.apache.org/) but leveraging the super data model to represent
a sequence of arbitrarily-typed values.
* [Super Columnar (CSUP)](csup.md) is columnar like
[Parquet](https://parquet.apache.org/),
[ORC](https://orc.apache.org/), or
[Arrow](https://arrow.apache.org/) but for super-structured data.
* [Super JSON (JSUP)](jsup.md) defines a format for encapsulating SUP
inside plain JSON for easy decoding by JSON-based clients, e.g.,
the [JavaScript library used by SuperDB Desktop](https://github.com/brimdata/zui/tree/main/packages/superdb-types)
and the [SuperDB Python library](../dev/libraries/python.md).

Because all of the formats conform to the same super-structured data model,
conversions between a human-readable form, a row-based binary form,
and a row-based columnar form can
be carried out with no loss of information.  This provides the best of both worlds:
the same data can be easily expressed in and converted between a human-friendly
and easy-to-program text form alongside efficient row and columnar formats.
