### Date/Times

>[!WARNING]
> These data types are going to change in a forthcoming release of SuperSQL.

The `time` type represents an unsigned 64-bit number of nanoseconds since epoch.

The `duration` type represents a signed 64-bit number of nanoseconds.

For backward compatibility with SQL, `INTERVAL` is a syntactic alias for type `duration`.

These data types are incompatible with SQL data and time types.  A future version
of SuperSQL will change the `time` type to be SQL compatible and add support for other
SQL date/time and interval types.  At that time, detailed syntax and semantics
for these types will be documented here.
