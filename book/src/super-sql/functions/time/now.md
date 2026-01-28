# now

the current time

## Synopsis

```
now() -> time
```

## Description

The `now` function takes no arguments and returns the current UTC time as a value of type `time`.

This is useful to timestamp events in a data pipeline, e.g.,
when generating errors that are marked with their time of occurrence:
```
switch (
  ...
  default ( values error({ts:now(), ...}) )
)
```

## Examples

_Running this command_
```
super -s -c 'values now()'
```
_produces this value (as of the last time this document was  updated)_
```
2025-08-27T02:18:53.568913Z
```
