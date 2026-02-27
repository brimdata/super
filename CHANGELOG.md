# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
