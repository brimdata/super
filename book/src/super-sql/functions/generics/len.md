# len

the type-dependent length of a value

## Synopsis

```
len(val: array|bytes|ip|map|net|null|record|set|string|type) -> int64
```

## Description

The _len_ function returns the length of its argument `val`.
The semantics of this length depend on the value's [type](../../types/intro.md).

For values of each of the supported types listed below, `len` describes the
contents of `val` as indicated.

|Type                             |What `len` Returns                                               |
|---------------------------------|-----------------------------------------------------------------|
|[`array`](../../types/array.md)  |Elements present                                                 |
|[`bytes`](../../types/bytes.md)  |Count of 8-bit bytes                                             |
|[`ip`](../../types/network.md)   |Bytes in the address (`4` for IPv4, `16` for IPv6)               |
|[`map`](../../types/map.md)      |Key-value pairs present                                          |
|[`net`](../../types/network.md)  |Bytes in the prefix and subnet mask (`8` for IPv4, `32` for IPv6)|
|[`null`](../../types/null.md)    |`0`                                                              |
|[`record`](../../types/record.md)|Fields present                                                   |
|[`set`](../../types/set.md)      |Elements present                                                 |
|[`string`](../../types/string.md)|Count of unicode code points                                     |

For values of the [`type`](../../types/type.md) type, `len` describes the
underlying type definition of `val` as indicated below.

|[Category](../types/kind.md) of `type` Value|What `len` Returns     |
|--------------------------------------------|-----------------------|
|`array`             |`len` of the defined element type              |
|`enum`              |Count of defined symbols                       |
|`error`             |`len` of the type of the defined wrapped values|
|`map`               |`len` of the defined value type                |
|`primitive`         |`1`                                            |
|`record`            |Count of defined fields                        |
|`set`               |`len` of the defined element type              |
|`union`             |Count of defined member types                  |

## Examples

---

_The length of values of various types_

```mdtest-spq {data-layout="stacked"}
# spq
values {this,kind:kind(this),type:typeof(this),len:len(this)}
# input
[1,2,3]
0x0102ffee
192.168.4.1
2001:0db8:85a3:0000:0000:8a2e:0370:7334
|{"APPL":145.03,"GOOG":87.07}|
192.168.4.0/24
2001:db8:abcd::/64
null
{a:1,b:2}
|["x","y","z"]|
"hello"
# expected output
{that:[1,2,3],kind:"array",type:<[int64]>,len:3}
{that:0x0102ffee,kind:"primitive",type:<bytes>,len:4}
{that:192.168.4.1,kind:"primitive",type:<ip>,len:4}
{that:2001:db8:85a3::8a2e:370:7334,kind:"primitive",type:<ip>,len:16}
{that:|{"APPL":145.03,"GOOG":87.07}|,kind:"map",type:<|{string:float64}|>,len:2}
{that:192.168.4.0/24,kind:"primitive",type:<net>,len:8}
{that:2001:db8:abcd::/64,kind:"primitive",type:<net>,len:32}
{that:null,kind:"primitive",type:<null>,len:0}
{that:{a:1,b:2},kind:"record",type:<{a:int64,b:int64}>,len:2}
{that:|["x","y","z"]|,kind:"set",type:<|[string]|>,len:3}
{that:"hello",kind:"primitive",type:<string>,len:5}
```

_The length of various values of type `type`_

```mdtest-spq {data-layout="stacked"}
# spq
values {this,kind:kind(this),type:typeof(this),len:len(this)}
# input
<[string]>
<[{a:int64,b:string,c:bool}]>
<enum(HEADS,TAILS)>
<error(string)>
<error({ts:time,msg:string})>
<|{string:float64}|>
<|{string:{x:int64,y:float64}}|>
<int8>
<{a:int64,b:string,c:bool}>
<|[string]|>
<|[{a:int64,b:string,c:bool}]|>
<(int64|float64|string)>
# expected output
{that:<[string]>,kind:"array",type:<type>,len:1}
{that:<[{a:int64,b:string,c:bool}]>,kind:"array",type:<type>,len:3}
{that:<enum(HEADS,TAILS)>,kind:"enum",type:<type>,len:2}
{that:<error(string)>,kind:"error",type:<type>,len:1}
{that:<error({ts:time,msg:string})>,kind:"error",type:<type>,len:2}
{that:<|{string:float64}|>,kind:"map",type:<type>,len:1}
{that:<|{string:{x:int64,y:float64}}|>,kind:"map",type:<type>,len:2}
{that:<int8>,kind:"primitive",type:<type>,len:1}
{that:<{a:int64,b:string,c:bool}>,kind:"record",type:<type>,len:3}
{that:<|[string]|>,kind:"set",type:<type>,len:1}
{that:<|[{a:int64,b:string,c:bool}]|>,kind:"set",type:<type>,len:3}
{that:<int64|float64|string>,kind:"union",type:<type>,len:3}
```

_Unsupported values produce [errors](../../types/error.md)_

```mdtest-spq {data-layout="stacked"}
# spq
values {this,kind:kind(this),type:typeof(this),len:len(this)}
# input
true
10m30s
error("hello")
1
2024-07-30T20:05:15.118252Z
# expected output
{that:true,kind:"primitive",type:<bool>,len:error({message:"len: bad type",on:true})}
{that:10m30s,kind:"primitive",type:<duration>,len:error({message:"len: bad type",on:10m30s})}
{that:error("hello"),kind:"error",type:<error(string)>,len:error({message:"len()",on:error("hello")})}
{that:1,kind:"primitive",type:<int64>,len:error({message:"len: bad type",on:1})}
{that:2024-07-30T20:05:15.118252Z,kind:"primitive",type:<time>,len:error({message:"len: bad type",on:2024-07-30T20:05:15.118252Z})}
```
