# Literals

Literal values represent specific instances of a [type](../types/intro.md) embedded directly
into an [expression](intro.md) like the integer `1`, the record `{x:1.5,y:-4.0}`,
or the mixed-type array `[1,"foo"]`.

Any valid [SUP](../../formats/sup.md) serialized text is a valid literal in SuperSQL.
In particular, complex-type expressions composed recursively of
other literal values can be used to construct any complex literal value,
e.g.,
* [record expressions](../types/record.md#record-expressions),
* [array expressions](../types/array.md#array-expressions),
* [set expressions](../types/set.md#set-expressions),
* [map expressions](../types/map.md#map-expressions), and
* [error expressions](../types/error.md).

Literal values of types
[enum](../types/enum.md),
[union](../types/union.md), and
[named](../types/named.md)
may be created with a [cast](cast.md).
