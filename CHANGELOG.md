# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-04-24

### **BREAKING CHANGE**

With the advance of the BSUP format to v4, `super` will no longer read BSUP v2
format as input. There is currently no backward compatibility in `super` for
reading BSUP v2 inputs. If you have valuable saved BSUP v2 files that you
cannot easily regenerate into v4 from their original sources, please speak up
on [community Slack](https://www.brimdata.io/join-slack/) or
[open an issue](https://github.com/brimdata/super/issues) for assistance.

### Added

- Vectorized JSON reader (#6764 #6789, #6847)
- Named types in `defuse` function (#6767, #6766)
- enum types in `upcast` (#6853)
- Type "none" for empty arrays/sets/maps (#6819)
- Recursive types (#6841)

### Changed

- Homebrew installation of `super` is now via `brew install super` rather than custom tap (#6796, #6835)
- Go 1.26 or later is now required to build `super` (#6778)
- Windows `super` release artifacts are now signed (#6761)
- Union types are now treated as sets and thus anonymous unions are no longer embeddable in another union (#6809)
- Named types are now immutable (#6821, #6790)
- Changed syntax for SUP type declarations, sets, and maps (#6828)
- The BSUP format has been advanced to version 4 (#6841)

### Removed

- The JSUP format and JSUP-based Python client are no longer supported (#6838)

### Fixed
- Fix handling of empty sets and map values in `unblend` function (#6770)
- Fix bug on record field in union type in `defuse` functon (#6771)
- Fix how the Arrow writer encodes a union of null and a nested union (#6772)
- Fix a panic that could occur when rendering certain query results as SUP (#6776)
- All named types are now preserved during fusion (#6777)
- Fix a race that could cause inconsistent/incomplete aggregation results on CSUP in vector runtime (#6782)
- Fix a bug in the CSUP union writer (#6784)
- Fix a `defuse` bug on none-valued optional record fields (#6786)
- Fix a bug with incorrect/inconsistent aggregations on JSON in vector runtime (#6793)
- Fix handling of optional fields with `missing` and `has` functions in vector runtime (#6807)
- Fix an issue in sequential runtime where `min`, `max`, or `sum` aggregate functions would incorrectly return null if their input consists entirely of unions (#6837)
- Fix an issue where the `coalesce` function did not correctly handle null and error values in vector runtime (#6836)
- Fix a panic with `distinct` in vector runtime (#6843)

## [0.3.0] - 2026-03-20

### **BREAKING CHANGE**

With the advance of the BSUP format to v2, `super` will no longer read BSUP v1
format as input. There is currently no backward compatibility in `super` for
reading BSUP v1 inputs. If you have valuable saved BSUP v1 files that you
cannot easily regenerate into v2 from their original sources, please speak up
on [community Slack](https://www.brimdata.io/join-slack/) or
[open an issue](https://github.com/brimdata/super/issues) for assistance.

### Added

- New `debug` operator (#6685, #6694, #6680, #6691, #6726)
- New `infer` operator (#6704)
- New `defuse` function (#6698)
- New `unblend` function (#6725)
- New `db vacate` command (#6706, #6747)
- Fusion types (#6713)
- Named types in `upcast` function (#6752)
- Optional fields in record expressions (#6702)

### Changed

- Change license to [SuperDB Source Available License v1.0](https://github.com/brimdata/super/blob/9343c50f2cdaf39ecfb3f90a458c552d3d0f8681/LICENSE.md) (#6755)
- In `collect` and `union` aggregate functions, `error("quiet")` values are now dropped and `error("missing")` values are preserved (#6710)
- `null` values are now ignored in `concat` function and f-strings (#6730)
- macOS `super` release artifacts are now signed and notarized (#6703)
- The BSUP format has been advanced to version 2 (#6713)

### Fixed

- Fix `unnest` bug when to-be-unnested array is in a union (#6692)
- Fix precision bug when casting from float16 or float32 to string (#6722)

## [0.2.0] - 2026-02-27

### **BREAKING CHANGE**

With the advance of the BSUP format to v1, `super` will no longer read BSUP v0
format as input. There is currently no backward compatibility in `super` for
reading BSUP v0 inputs. If you have valuable saved BSUP v0 files that you
cannot easily regenerate into v1 from their original sources, please speak up
on [community Slack](https://www.brimdata.io/join-slack/) or
[open an issue](https://github.com/brimdata/super/issues) for assistance.

### Added

- New `upcast` function (#6634)
- Line numbers are now shown for SUP parsing errors (#6630)
- Optional fields in records (#6669)

### Changed

- Upgrade to `github.com/apache/arrow-go/v18@v18.5.1` (#6625)
- Type fusion now ensures any fused unions contain at most one instance of each kind of complex type (#6651)
- Type decoration for self-describing error values in SUP are now omitted (#6656)
- By default, `super` now reads the first 1000 values when reading from an input file to infer type information used to type check the query. This can cause type errors when data being referenced occurs later in the input. The `-samplesize` flag can be used to adjust this. (#6667)
- The BSUP format has been advanced to version 1 (#6674)

### Removed

- Typed nulls are no longer supported, with nullable values now represented with a union that includes type `null` (#6633)

### Fixed

- vam: Casting an error value now propagates the error (#6602)
- vam: Fix an issue where a mixed type error was not returned if the aggregation received a number first followed by a string (#6618)
- vam: Fix a `rename` operator issue where nested records were not getting assigned updated types (#6623)
- Fix a deadlock that could occur when running a group-by aggregation on BSUP input (#6624)
- The BSUP union tag is now encoded as a uvarint, in line with the specification (#6660)

## [0.1.0] - 2026-01-30

### Added

- Initial release (see the [project documentation](https://superdb.org/intro.html))
