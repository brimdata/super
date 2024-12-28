### Operator

&emsp; **drop** &mdash; drop fields from record values

### Synopsis

```
drop <field> [, <field> ...]
```
### Description

The `drop` operator removes one or more fields from records in the input sequence
and copies the modified records to its output.  If a field to be dropped
is not present, then no effect for the field occurs.  In particular,
non-record values are copied unmodified.

### Examples

_Drop of a field_
```mdtest-command
echo '{a:1,b:2,c:3}' | super -z -c 'drop b' -
```

```mdtest-output
{a:1,c:3}
```
_Non-record values are copied to output_
```mdtest-command
echo '1 {a:1,b:2,c:3}' | super -z -c 'drop a,b' -
```

```mdtest-output
1
{c:3}
```
