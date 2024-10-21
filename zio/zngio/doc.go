// Package zngio provides an API for reading and writing zng values and
// directives in binary zng format.  The Reader and Writer types implement the
// the zio.Reader and zio.Writer interfaces.  Since these methods
// read and write only super.Records, but the zng format includes additional
// functionality, other methods are available to read/write zng comments
// and include virtual channel numbers in the stream.  Virtual channels
// provide a way to indicate which output of a flowgraph a result came from
// when a flowgraph computes multiple output channels.  The zng values in
// this zng value are "machine format" as prescirbed by the ZNG spec.
// The vanilla zio.Reader and zio.Writer implementations ignore application-specific
// payloads (e.g., channel encodings).
package zngio
