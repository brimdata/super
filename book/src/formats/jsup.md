# Super JSON (JSUP) Format

## 1. Introduction

The [super-structured data model](model.md)
is based on richly typed records with a deterministic field order,
as is implemented by the [Super (SUP)](sup.md), [Super Binary (BSUP)](bsup.md), and [Super Columnar (CSUP)](csup.md) formats.

Given the ubiquity of JSON, it is desirable to also be able to serialize
super data into the JSON format.   However, encoding super data values
directly as JSON values would not work without loss of information.

For example, consider this [SUP](sup.md) data:
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
Also, it is at the whim of a JSON implementation whether
or not the order of object keys is preserved.

While JSON is well suited for data exchange of generic information, it is not
sufficient for super-structured data.
That said, JSON can be used as an encoding format for super-structured data with another layer
of encoding on top of a JSON-based protocol.  This allows clients like web apps or
Electron apps to receive and understand SUP and, with the help of client
libraries like [superdb-types](https://github.com/brimdata/zui/tree/main/packages/superdb-types),
to manipulate the rich, structured SUP types that are implemented on top of
the basic JavaScript types.

In other words,
because JSON objects do not have a deterministic field order nor does JSON
in general have typing beyond the basics (i.e., strings, floating point numbers,
objects, arrays, and booleans), SUP and
its embedded type model is layered on top of regular JSON.

## 2. The Format

The format for representing SUP data in JSON is called JSUP.
Converting SUP, BSUP, or CSUP to JSUP and back results in a complete and
accurate restoration of the original super data.

A JSUP stream is defined as a sequence of JSON objects where each object
represents a value and has the form:
```
{
  "type": <type>,
  "value": <value>
}
```
The type and value fields are encoded as defined below.

### 2.1 Type Encoding

The type encoding for a primitive type is simply its [type name](model.md#1-primitive-types)
e.g., "int32" or "string".

Complex types are encoded with small-integer identifiers.
The first instance of a unique type defines the binding between the
integer identifier and its definition, where the definition may recursively
refer  to earlier complex types by their identifiers.

For example, the type `{s:string,x:int32}` has this JSUP format:
```
{
  "id": 123,
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
        "name": "int32"
      }
    }
  ]
}
```

A previously defined complex type may be referred to using a reference of the form:
```
{
  "kind": "ref",
  "id": 123
}
```

#### 2.1.1 Record Type

A record type is a JSON object of the form
```
{
  "id": <number>,
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
and `<name>` is a string defining the field name and `<type>` is a
recursively encoded type.

#### 2.1.2 Array Type

An array type is defined by a JSON object having the form
```
{
  "id": <number>,
  "kind": "array",
  "type": <type>
}
```
where `<type>` is a recursively encoded type.

#### 2.1.3 Set Type

A set type is defined by a JSON object having the form
```
{
  "id": <number>,
  "kind": "set",
  "type": <type>
}
```
where `<type>` is a recursively encoded type.

#### 2.1.4 Map Type

A map type is defined by a JSON object of the form
```
{
  "id": <number>,
  "kind": "map",
  "key_type": <type>,
  "val_type": <type>
}
```
where each `<type>` is a recursively encoded type.

#### 2.1.5 Union type

A union type is defined by a JSON object having the form
```
{
  "id": <number>,
  "kind": "union",
  "types": [ <type>, <type>, ... ]
}
```
where the list of types comprise the types of the union and
and each `<type>`is a recursively encoded type.

#### 2.1.6 Enum Type

An enum type is a JSON object of the form
```
{
  "id": <number>,
  "kind": "enum",
  "symbols": [ <string>, <string>, ... ]
}
```
where the unique `<string>` values define a finite set of symbols.

#### 2.1.7 Error Type

An error type is a JSON object of the form
```
{
  "id": <number>,
  "kind": "error",
  "type": <type>
}
```
where `<type>` is a recursively encoded type.

#### 2.1.8 Named Type

A named type is encoded as a binding between a name and a type
and represents a new type so named.  A type definition type has the form
```
{
  "id": <number>,
  "kind": "named",
  "name": <id>,
  "type": <type>,
}
```
where `<id>` is a JSON string representing the newly defined type name
and `<type>` is a recursively encoded type.

### 2.2 Value Encoding

The primitive values comprising an arbitrarily complex data value are encoded
as a JSON array of strings mixed with nested JSON arrays whose structure
conforms to the nested structure of the value's schema as follows:
* each record, array, and set is encoded as a JSON array of its composite values,
* a union is encoded as a string of the form `<tag>:<value>` where `tag`
is an integer string representing the positional index in the union's list of
types that specifies the type of `<value>`, which is a JSON string or array
as described recursively herein,
* a map is encoded as a JSON array of two-element arrays of the form
`[ <key>, <value> ]` where `key` and `value` are recursively encoded,
* a type value is encoded [as above](#21-type-encoding),
* each primitive that is not a type value
is encoded as a string conforming to its SUP representation, as described in the
[corresponding section of the SUP specification](sup.md#23-primitive-values).

For example, a record with three fields --- a string, an array of integers,
and an array of union of string, and float64 --- might have a value that looks like this:
```
[ "hello, world", ["1","2","3","4"], ["1:foo", "0:10" ] ]
```

## 3. Object Framing

A JSUP file is composed of JSUP objects formatted as
[newline delimited JSON (NDJSON)](https://en.wikipedia.org/wiki/JSON_streaming#NDJSON).
e.g., the [`super`](../command/super.md) CLI command
writes its JSUP output as lines of NDJSON.

## 4. Example

Here is an example that illustrates values of a repeated type,
nesting, records, array, and union. Consider the file `input.sup`:

```mdtest-input input.sup
{s:"hello",r:{a:1,b:2}}
{s:"world",r:{a:3,b:4}}
{s:"hello",r:{a:[1,2,3]}}
{s:"goodnight",r:{x:{u:"foo"::(string|int64)}}}
{s:"gracie",r:{x:{u:12::(string|int64)}}}
```

This data is represented in JSUP as follows:

```mdtest-command
super -f jsup input.sup | jq .
```

```mdtest-output
{
  "type": {
    "kind": "record",
    "id": 31,
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
          "id": 30,
          "fields": [
            {
              "name": "a",
              "type": {
                "kind": "primitive",
                "name": "int64"
              }
            },
            {
              "name": "b",
              "type": {
                "kind": "primitive",
                "name": "int64"
              }
            }
          ]
        }
      }
    ]
  },
  "value": [
    "hello",
    [
      "1",
      "2"
    ]
  ]
}
{
  "type": {
    "kind": "ref",
    "id": 31
  },
  "value": [
    "world",
    [
      "3",
      "4"
    ]
  ]
}
{
  "type": {
    "kind": "record",
    "id": 34,
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
          "id": 33,
          "fields": [
            {
              "name": "a",
              "type": {
                "kind": "array",
                "id": 32,
                "type": {
                  "kind": "primitive",
                  "name": "int64"
                }
              }
            }
          ]
        }
      }
    ]
  },
  "value": [
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
  "type": {
    "kind": "record",
    "id": 38,
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
          "id": 37,
          "fields": [
            {
              "name": "x",
              "type": {
                "kind": "record",
                "id": 36,
                "fields": [
                  {
                    "name": "u",
                    "type": {
                      "kind": "union",
                      "id": 35,
                      "types": [
                        {
                          "kind": "primitive",
                          "name": "int64"
                        },
                        {
                          "kind": "primitive",
                          "name": "string"
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
  },
  "value": [
    "goodnight",
    [
      [
        [
          "1",
          "foo"
        ]
      ]
    ]
  ]
}
{
  "type": {
    "kind": "ref",
    "id": 38
  },
  "value": [
    "gracie",
    [
      [
        [
          "0",
          "12"
        ]
      ]
    ]
  ]
}
```
