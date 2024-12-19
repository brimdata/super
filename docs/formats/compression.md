---
weight: 6
title: Compression
heading: ZNG Compression Types
---

This document specifies values for the `<format>` byte of a
[Super Binary compressed value message block](bsup.md#2-the-super-binary-format)
and the corresponding algorithms for the `<compressed payload>` byte sequence.

As new compression algorithms are specified, they will be documented
here without any need to change the ZNG specification.

Of the 256 possible values for the `<format>` byte, only type `0` is currently
defined and specifies that `<compressed payload>` contains an
[LZ4 block](https://github.com/lz4/lz4/blob/master/doc/lz4_Block_format.md).
