# Formats

The supported [input](input.md) and [output](output.md) formats include the following:

|  Option   | Auto | Extension | Specification                            |
|-----------|------|-----------|------------------------------------------|
| `arrows`  |  yes | `.arrows` | [Arrow IPC Stream Format](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format) |
| `bsup`    |  yes | `.bsup` | [BSUP](../formats/bsup.md) |
| `csup`    |  yes | `.csup` | [CSUP](../formats/csup.md) |
| `csv`     |  yes | `.csv` | [Comma-Separated Values (RFC 4180)](https://www.rfc-editor.org/rfc/rfc4180.html) |
| `json`    |  yes | `.json` | [JSON (RFC 8259)](https://www.rfc-editor.org/rfc/rfc8259.html) |
| `jsup`   |  yes | `.jsup` | [Super over JSON (JSUP)](../formats/jsup.md) |
| `line`    |  no  | n/a | One text value per line |
| `parquet` |  yes | `.parquet` | [Apache Parquet](https://github.com/apache/parquet-format) |
| `sup`     |  yes | `.sup` | [SUP](../formats/sup.md) |
| `tsv`     |  yes | `.tsv` | [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values) |
| `zeek`    |  yes | `.zeek` | [Zeek Logs](https://docs.zeek.org/en/master/logs/index.html) |

>[!NOTE]
> Best performance is typically achieved when operating on data in binary columnar formats
> such as [CSUP](../formats/csup.md),
> [Parquet](https://github.com/apache/parquet-format), or
> [Arrow](https://arrow.apache.org/docs/format/Columnar.html#ipc-streaming-format).
