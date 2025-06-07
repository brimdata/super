# Introduction

TODO: WRITE THIS SECTION

TODO: motivate with wrangling use case (need other systems generally or JSON columns)
then why not use the same for analytics as wrangling?

SuperDB is a new analytics database that unifies structured and semi-structured data
into a superset of these disparate data models called super-structured data.
This makes dealing with modern eclectic data easier because relational data and JSON data
are treated the same way from the ground up.

i.e., relational tables and JSON data are treated the same a

with its super-structured data model.

XXX clarify db vs super, connect to an instance vs operate directly on inputs

XXX currently no support for connecting to relational systems and open Lake formats,
but that may come...

XXX In a block quote:
XXX ref PRQL, Against SQL, Google Pipes paper, sane QL paper, SQL++

XXX discuss types scaling

TODO: where does this go?
```sh
; duckdb -c "select [1,'foo']"    
Conversion Error:
Could not convert string 'foo' to INT32

LINE 1: select [1,'foo']
                  ^
; clickhouse -q "select [1,'foo']"    
Code: 386. DB::Exception: There is no supertype for types UInt8, String because some of them are String/FixedString/Enum and some of them are not. (NO_COMMON_TYPE)
; datafusion-cli -c "select [1,'foo']"  
DataFusion CLI v46.0.1
Error: Arrow error: Cast error: Cannot cast string 'foo' to value of Int64 type
; ~/demo/zeta/execute_query_macos "select [1,'foo']"
Array elements of types {INT64, STRING} do not have a common supertype
```


