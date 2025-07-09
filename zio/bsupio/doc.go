// Package bsupio provides an API for reading and writing super values and
// types in binary BSUP format.  Reader and Writer implement the
// the zio.Reader and zio.Writer interfaces.  Since these methods
// read and write only super.Values, but the BSUP format includes additional
// functionality, other methods are available to read/write BSUP control
// messages in the stream.  Virtual channels provide a way to indicate
// which output of a flowgraph a result came from
// when a flowgraph computes multiple output channels.  The BSUP values in
// this super.Value are "machine format" as prescirbed by the BSUP spec.
// The vanilla zio.Reader and zio.Writer implementations ignore application-specific
// payloads (e.g., channel encodings).
package bsupio
