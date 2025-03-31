// Package zngio provides an API for reading and writing super values and
// types in binary BSON format.  Reader and Writer implement the
// the zio.Reader and zio.Writer interfaces.  Since these methods
// read and write only super.Values, but the BSON format includes additional
// functionality, other methods are available to read/write BSON control
// messages in the stream.  Virtual channels provide a way to indicate
// which output of a flowgraph a result came from
// when a flowgraph computes multiple output channels.  The BSON values in
// this super.Value are "machine format" as prescirbed by the BSON spec.
// The vanilla zio.Reader and zio.Writer implementations ignore application-specific
// payloads (e.g., channel encodings).
package zngio
