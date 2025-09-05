### Function

&emsp; **date_part** &mdash; return a specified part of a time value

### Synopsis

```
date_part(part: string, ts: time) -> int64
```

### Description

The `date_part` function accepts a [`string`](../../types/string.md) argument `part` and a [`time`](../../types/time.md) value `ts` and
returns an [`int64`](../../types/numbers.md) representing the part of the date requested.

Valid values for `part` are:

|`part` Value            |Returned by `date_part`                                                                                    |
|:-----------------------|:----------------------------------------------------------------------------------------------------------|
|`"day"`                 |The day of the month (1-31)                                                                                |
|`"dow"`<br>`"dayofweek"`|The day of the week (0-6; Sunday is 0)                                                                     |
|`"hour"`                |The hour field (0-23)                                                                                      |
|`"microseconds"`        |The seconds field but in microseconds including fractional parts (i.e., 1 second is 1,000,000 microseconds)|
|`"milliseconds"`        |The seconds field but in milliseconds including fractional parts (i.e., 1 second is 1,000 milliseconds)    |
|`"minute"`              |The minute field (0-59)                                                                                    |
|`"month"`               |The month of the year (1-12)                                                                               |
|`"second"`              |The seconds field (0-59)                                                                                   |
|`"year"`                |The year field                                                                                             |

### Examples

---

_Extract all individual parts from a `time` value_

```mdtest-spq
# spq
values date_part("day", this),
       date_part("dow", this),
       date_part("hour", this),
       date_part("microseconds", this),
       date_part("milliseconds", this),
       date_part("minute", this),
       date_part("month", this),
       date_part("second", this),
       date_part("year", this)
# input
2001-10-09T01:46:40.123456Z
# expected output
9
2
1
40123456
40123
46
10
40
2001
```
