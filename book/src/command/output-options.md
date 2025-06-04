### Output Options

* `-B` allow Super Binary to be sent to a terminal output
* `-bsup.compress` compress Super Binary frames
* `-bsup.framethresh` minimum Super Binary frame size in uncompressed bytes (default "524288")
* `-bsup.readmax` maximum Super Binary read buffer size in MiB, MB, etc.
* `-bsup.readsize` target Super Binary read buffer size in MiB, MB, etc.
* `-bsup.threads` number of Super Binary read threads
* `-bsup.validate` validate format when reading Super Binary
* `-color` enable/disable color formatting for -S and db text output
* `-csv.delim` CSV field delimiter
* `-f` format for output data
* `-J` shortcut for `-f json -pretty`, i.e., multi-line JSON
* `-j` shortcut for `-f json -pretty=0`, i.e., line-oriented JSON
* `-o` write data to output file
* `-persist` regular expression to persist type definitions across the stream
* `-pretty` tab size to pretty print JSON and Super JSON output
* `-S` shortcut for `-f sup -pretty`, i.e., multi-line SUP
* `-s` shortcut for `-f sup -pretty=0`, i.e., line-oriented SUP
* `-split split` output into one file per data type in this directory
* `-splitsize` if >0 and -split is set, split into files at least this big rather than by data type
* `-unbuffered` disable output buffering
