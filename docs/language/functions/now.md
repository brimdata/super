### Function

&emsp; **now** &mdash; the current time

### Synopsis

```
now() -> time
```

### Description

The _now_ function takes no arguments and returns the current UTC time as a value of type `time`.

This is useful to timestamp events in a data pipeline, e.g.,
when generating errors that are marked with their time of occurrence:
```
switch (
  ...
  default => yield error({ts:now(), ...})
)
```

### Examples

```
super -s -c 'yield now()'
```
=>
```
2025-01-24T16:57:20.555988Z
```
(at the time this document was last updated)
