### Function

&emsp; **date_part** &mdash; return a specified part of a time value

### Synopsis

```
date_part(part: string, ts: time) -> int64
```

### Description

The `date_part` function accepts a string `part` argument and a time value `ts` and
returns an int64 representing the part of the date requested.

Valid values for `part` are:

- "day": The day of the month (1-31).
- "dow" / "dayofweek": The day of the week (0-6; Sunday is 0).
- "hour": The hour field (0-23).
- "microseconds": The seconds field but in microseconds including fractional parts (i.e., 1 second is 1,000,000 microseconds).
- "milliseconds": The seconds field but in milliseconds including fractional parts (i.e., 1 second is 1,000 milliseconds).
- "minute": The minute field (0-59).
- "month": The month of the year (1-12).
- "second": The seconds field (0-59).
- "year": The year field.

### Examples

---

_Extract the year, month, and day of the month from a time value_

```mdtest-spq
# spq
values date_part("year", this), date_part("month", this), date_part("day", this)
# input
2001-09-09T01:46:40Z
# expected output
2001
9
9
```
