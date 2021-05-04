# Zed over JSON

* [ZJSON](#zjson)
  + [Type Encoding](#type-encoding)
    - [Record Type](#record-type)
    - [Array Type](#array-type)
    - [Set Type](#set-type)
    - [Union type](#union-type)
    - [Enum Type](#enum-type)
    - [Type Definition](#type-definition)
    - [Type Name](#type-name)
  + [Value Encoding](#value-encoding)
* [Framing ZJSON objects](#framing-zjson-objects)
* [Example](#example)

The Zed data model is based on richly typed records with a deterministic column order,
as is implemented by the ZSON, ZNG, and ZST formats.
Given the ubiquity of JSON, it is desirable to also be able to serialize
Zed data into the JSON format.   However, encoding Zed data values
directly as JSON values would not work without loss of information.

For example, consider this Zed data as [ZSON](zson.md):
```
{
    ts: 2018-03-24T17:15:21.926018012Z,
    a: "hello, world",
    b: {
        x: 4611686018427387904,
        y: 127.0.0.1
    }
}
```
A straightforward translation to JSON might look like this:
```
{
  "ts": 1521911721.926018012,
  "a": "hello, world",
  "b": {
    "x": 4611686018427387904,
    "y": "127.0.0.1"
  }
}
```
But, when this JSON is transmitted to a JavaScript client and parsed,
the result looks something like this:
```
{
  "ts": 1521911721.926018,
  "a": "hello, world",
  "b": {
    "x": 4611686018427388000,
    "y": "127.0.0.1"
  }
}
```
The good news is the `a` field came through just fine, but there are
a few problems with the remaining fields:
* the timestamp lost precision (due to 53 bits of mantissa in a JavaScript
IEEE 754 floating point number) and was converted from a time type to a number,
* the int64 lost precision for the same reason, and
* the IP address has been converted to a string.

As a comparison, Python's `json` module handles the 64-bit integer to full
precision, but loses precision on the floating point timestamp.
Also, as mentioned, it is at the whim of a JSON implementation whether
or not the order of object keys is preserved.

While JSON is well suited for data exchange of generic information, it is not
so appropriate for a structured data model like Zed.
That said, JSON can be used as an encoding format for Zed by mapping Zed data
onto a JSON-based protocol.  This allows clients like web apps or
Electron apps to receive and understand Zed and, with the help of client
libraries like [zealot](https://github.com/brimdata/brim/tree/master/zealot),
to manipulate the rich, structured Zed types that are implemented on top of
the basic JavaScript types.

In other words,
because JSON objects do not have a deterministic column order nor does JSON
in general have typing beyond the basics (i.e., strings, floating point numbers,
objects, arrays, and booleans), we decided to encode Zed data with
its embedded type model all in a layer above regular JSON.

## ZJSON

The format for representing Zed in JSON is called ZJSON.
Converting ZSON/ZNG/ZST to ZJSON and back results in a complete and
accurate restoration of the original Zed data.

The ZJSON data model follows that of the underlying Zed model by embedding
type information in the stream: type definitions declare arbitrarily complex
and nested data types, and values are sent referencing the type information
recursively with small-integer type identifiers.

Since Zed steams are self describing and type information is embedded
in the stream itself, the embedded types are likewise encoded in the
ZJSON format.

A ZJSON stream is defined as a sequence of JSON objects where each object
represents a Zed value.  Each object includes an identifier that denotes
its type, or _schema_.  A schema generically refers to the type of the
Zed value that is defined by a given JSON object.

Each object contains the following fields:
* `schema` a name encoded as a JSON string indicating the type that
applies to this value where the definition for the type appears in the
types field or in a previous occurrence of the types field in the stream,
* `values` a JSON array of strings and arrays encoded as defined below,
* `types` a JSON array of types where the types contain "TypeDef" types
establishing a binding between the names referred to by the `schema` fied.

The schema name provides a mapping to a type so that future values in the stream may
reference a type by name.  An implementation maintains a table to map schema names
to types as it decodes values.  The names are scoped to the particular ZJSON
data stream in which they are embedded and otherwise have no global persistence
or meaning.

Objects in a ZJSON stream have the following JSON structure:
```
{
  "schema": <id>,
  "values": [ <val>, ... [ <val>, ... ] ... ]
  "types": [ <type>, <type>, ... ]
}
```

### Type Encoding

The type format follows the terminology in the [ZSON spec](zson.md), where primitive types
represent concrete values like strings, integers, times, and so forth, while
complex types are composed of primitive types and/or other complex types, e.g.,
records, sets, arrays, and unions.

The ZJSON type encoding for a primitive type is simply its ZSON string name,
e.g., "int32" or "string".  Complex types are structured and their
mapping onto JSON depends on the type.  For example,
the Zed type `{s:string,x:int32}` has this ZJSON format:
```
{
  "kind": "record",
  "fields": [
    {
      "name": "s",
      "type": {
        "kind": "primitive",
        "name": "string"
      }
    },
    {
      "name": "x",
      "type": {
        "kind": "primitive",
        "name": "int64"
      }
    }
  ]
}
```
A type string may also contain a type name previously defined by a type definition.

#### Record Type

More formally, a Zed record type is a JSON object of the form
```
{
  "kind": "record",
  "fields": [ <field>, <field>, ... ]
}
```
where each of the fields has the form
```
{
  "name": <name>,
  "type": <type>,
}
```
and `<name>` is a string defining the column name and `<type>` is a
recursively encoded type.

#### Array Type

A Zed array type is defined by a JSON object having the form
```
{
  "kind": "array",
  "type": <type>
}
```
where `<type>` is a recursively encoded type.

#### Set Type

A Zed set type is defined by a JSON object having the form
```
{
  "kind": "set",
  "type": <type>
}
```
where `<type>` is a recursively encoded type.

#### Union type

A Zed union type is defined by a JSON object having the form
```
{
  "kind": "union",
  "types": [ <type>, <type>, ... ]
}
```
where the list of types comprise the types of the union and
and each `<type>`is a recursively encoded type.

#### Map Type

A Zed map type is defined by a JSON object of the form
```
{
  "kind": "map",
  "key_type": <type>,
  "val_type": <type>
}
```

#### Enum Type

A Zed enum type is a JSON object of the form
```
{
  "kind": "enum",
  "symbols": [ <string>, <string>, ... ]
}
```

#### Type Definition

A type definition is encoded as a binding between a name and a Zed type
and represents a new type so named.  A type definition type has the form
```
{
  "kind": "typedef",
  "name": <id>,
  "type": <type>,
}
```
where `<id>` is a JSON string representing the newly defined type name
and `<type>` is a recursively encoded type.
If `<id>` is a non-integer string, then it is a user-visible
type name.  If it is an integer string, then it is not user-visible and is used  
exclusively to correlate first-class Zed type values in a values array with
their corresponding type.

#### Type Name

A type reference is encoded as a reference to a previously defined type definition
and has the form
```
{
  "kind": "typename",
  "name": <id>,
}
```
where `<id>` is a JSON string representing a previously defined type name.

### Value Encoding

The primitive values comprising an arbitrarily complex Zed data value are encoded
as a JSON array of strings mixed with nested JSON arrays whose structure
conforms to the nested structure of the value's schema as follows:
* each record, array, and set is encoded as a JSON array of its composite values,
* a union is encoded as a string of the form `<selector>:<value>>` where `selector`
is an integer string representing the positional index in the union's list of
types that specifies the type of `<value>`, which is a JSON string or array
as described recursively herein,
a map is encoded as a JSON array of two-element arrays of the form
`[ <key>, <value> ]` where `key` and `value` are recursively encoded,
* a type value is encoded as:
    * its primitive type name for primitive types, or
    * its typedef name as defined in a present or previous types array  in
      the top-level object stream,
* each primitive that is not a type value
is encoded as a string conforming to its ZSON representation, as described in the
[corresponding section of the ZSON specification](zson.md#33-primitive-values).

For example, a record with three columns --- a string, an array of integers,
and an array of union of string, and float64 --- might have a value that looks like this:
```
[ "hello, world", ["1","2","3","4"], ["1:foo", "0:10" ] ]
```

## Framing ZJSON objects

A sequence of ZJSON objects may be framed in two primary ways.

First, they can simply be [newline delimited JSON (NDJSON)](http://ndjson.org/), where
each object is transmitted as a single line terminated with a newline character,
e.g., the [zq](https://github.com/brimdata/zed/tree/main/cmd/zq) CLI command writes its
ZJSON output as lines of NDJSON.

Second, the objects may be encoded in a JSON array embedded in some other
JSON-framed protocol, e.g., embedded in the the search results messages
of the [zqd REST API](../../api/api.go).

It is up to an implementation to determine how ZJSON
objects are framed according to its particular use case.

## Example

Here is an example that illustrates values of a repeated type,
nesting, records, array, and union:

```
{s:"hello",r:{a:1 (int32),b:2 (int32)} (=0)} (=1)
{s:"world",r:{a:3,b:4}} (1)
{s:"hello",r:{a:[1 (int32),2 (int32),3 (int32)] (=2)} (=3)} (=4)
{s:"goodnight",r:{x:{u:"foo" (5=((string,int32)))} (=6)} (=7)} (=8)
{s:"gracie",r:{x:{u:12 (int32)}}} (8)
```

This data is represented in ZJSON as follows:

```
{
  "schema": "24",
  "types": [
    {
      "kind": "typedef",
      "name": "24",
      "type": {
        "kind": "record",
        "fields": [
          {
            "name": "s",
            "type": {
              "kind": "primitive",
              "name": "string"
            }
          },
          {
            "name": "r",
            "type": {
              "kind": "record",
              "fields": [
                {
                  "name": "a",
                  "type": {
                    "kind": "primitive",
                    "name": "int32"
                  }
                },
                {
                  "name": "b",
                  "type": {
                    "kind": "primitive",
                    "name": "int32"
                  }
                }
              ]
            }
          }
        ]
      }
    }
  ],
  "values": [
    "hello",
    [
      "1",
      "2"
    ]
  ]
}
{
  "schema": "24",
  "values": [
    "world",
    [
      "3",
      "4"
    ]
  ]
}
{
  "schema": "27",
  "types": [
    {
      "kind": "typedef",
      "name": "27",
      "type": {
        "kind": "record",
        "fields": [
          {
            "name": "s",
            "type": {
              "kind": "primitive",
              "name": "string"
            }
          },
          {
            "name": "r",
            "type": {
              "kind": "record",
              "fields": [
                {
                  "name": "a",
                  "type": {
                    "kind": "array",
                    "type": {
                      "kind": "primitive",
                      "name": "int32"
                    }
                  }
                }
              ]
            }
          }
        ]
      }
    }
  ],
  "values": [
    "hello",
    [
      [
        "1",
        "2",
        "3"
      ]
    ]
  ]
}
{
  "schema": "31",
  "types": [
    {
      "kind": "typedef",
      "name": "31",
      "type": {
        "kind": "record",
        "fields": [
          {
            "name": "s",
            "type": {
              "kind": "primitive",
              "name": "string"
            }
          },
          {
            "name": "r",
            "type": {
              "kind": "record",
              "fields": [
                {
                  "name": "x",
                  "type": {
                    "kind": "record",
                    "fields": [
                      {
                        "name": "u",
                        "type": {
                          "kind": "union",
                          "types": [
                            {
                              "kind": "primitive",
                              "name": "string"
                            },
                            {
                              "kind": "primitive",
                              "name": "int32"
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
  ],
  "values": [
    "goodnight",
    [
      [
        [
          "0",
          "foo"
        ]
      ]
    ]
  ]
}
{
  "schema": "31",
  "values": [
    "gracie",
    [
      [
        [
          "1",
          "12"
        ]
      ]
    ]
  ]
}
```
