package anyio

import "io"

const TrackSize = InitBufferSize

type Track struct {
	rs      io.ReadSeeker
	initial int64

	recorder *Recorder
	off      int
}

func NewTrack(r io.Reader) *Track {
	if rs, ok := r.(io.ReadSeeker); ok {
		if n, err := rs.Seek(0, io.SeekCurrent); err == nil {
			return &Track{rs: rs, initial: n}
		}
	}
	return &Track{
		recorder: NewRecorder(r),
	}
}

func (t *Track) Reset() {
	if t.rs != nil {
		// We ignore errors here under the assumption that a subsequent
		// call to Read will also fail.
		t.rs.Seek(t.initial, io.SeekStart)
		return
	}
	t.off = 0
}

func (t *Track) Read(b []byte) (int, error) {
	if t.rs != nil {
		return t.rs.Read(b)
	}
	n, err := t.recorder.ReadAt(t.off, b)
	t.off += n
	return n, err
}

func (t *Track) Reader() io.Reader {
	if t.rs != nil {
		t.Reset()
		return t.rs
	}
	return t.recorder
}
