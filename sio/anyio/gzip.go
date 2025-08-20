package anyio

import (
	"compress/gzip"
	"io"
)

func GzipReader(r io.Reader) (io.Reader, error) {
	if rs, ok := r.(io.ReadSeeker); ok {
		if n, err := rs.Seek(0, io.SeekCurrent); err == nil {
			if r, err := gzip.NewReader(rs); err == nil {
				return r, nil
			}
			if _, err := rs.Seek(n, io.SeekStart); err != nil {
				return nil, err
			}
			return rs, nil
		}
	}
	track := NewTrack(r)
	// gzip.NewReader blocks until it reads ten bytes.  readGzipID only
	// reads two bytes.
	if !readGzipID(track) {
		return track.Reader(), nil
	}
	track.Reset()
	_, err := gzip.NewReader(track)
	if err == nil {
		return gzip.NewReader(track.Reader())
	}
	return track.Reader(), nil
}

// RFC 1952, Section 2.3.1
const (
	gzipID1 = 0x1f
	gzipID2 = 0x8b
)

// readGzipID returns true if it can read gzipID1 followed by gzipID2 from r.
// It reads exactly two bytes from r.
func readGzipID(r io.Reader) bool {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return false
	}
	return buf[0] == gzipID1 && buf[1] == gzipID2
}
