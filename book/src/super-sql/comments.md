## Comments

TODO: update this old text (SQL comments)
TODO: let's get multiline comments working

To further ease the maintenance and readability of source files, comments
beginning with `--` may appear in SuperSQL query texts.

```
-- This includes a search with boolean logic, an expression, and an aggregation.

search "example.com" AND "urgent"
| where message_length > 100       // We only care about long messages
| aggregate kinds:=union(type) by net:=network_of(srcip)
```
