# ZNG Compression Format Specification

This document specifies values for the `<format>` field of a
[ZNG compressed value message block](./spec.md#313-compressed-value-message-block)
and the corresponding algorithms for the `<compressed-messages>` field.

A `<format>` of `0` specifies that `<compressed-messages>` contains an
[LZ4 block](https://github.com/lz4/lz4/blob/master/doc/lz4_Block_format.md).
