package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/brimdata/super"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr/extent"
	"github.com/segmentio/ksuid"
)

const (
	DefaultSeekStride = 64 * 1024
	DefaultThreshold  = 500 * 1024 * 1024
)

// A FileKind is the first part of a file name, used to differentiate files
// when they are listed from the archive's backing store.
type FileKind string

const (
	FileKindUnknown  FileKind = ""
	FileKindData     FileKind = "data"
	FileKindMetadata FileKind = "meta"
	FileKindSeek     FileKind = "seek"
)

func (k FileKind) Description() string {
	switch k {
	case FileKindData:
		return "data"
	case FileKindMetadata:
		return "metadata"
	case "FileKindSeek":
		return "seekindex"
	default:
		return "unknown"
	}
}

//XXX all file types are cacheable but seek index etc is not matched here.
//and seekindexes really do want to be cached as they are small and
// eliminate round-trips, especially when you are ready sub-ranges of
// cached data files!

var fileRegex = regexp.MustCompile(`([0-9A-Za-z]{27}-(data|meta)).bsup$`)

// XXX this won't work right until we integrate segID
func FileMatch(s string) (kind FileKind, id ksuid.KSUID, ok bool) {
	match := fileRegex.FindStringSubmatch(s)
	if match == nil {
		return
	}
	k := FileKind(match[1])
	switch k {
	case FileKindData:
	case FileKindMetadata:
	default:
		return
	}
	id, err := ksuid.Parse(match[2])
	if err != nil {
		return
	}
	return k, id, true
}

// An Object represents a cloud object or file that holds an ordered sequence
// of values sorted according to the pool's data order where From is the
// the first value in the sequence and To is the last value.  Count is the number
// of values in the sequence and Size is total size in bytes of the Object as
// persisted to storage (i.e., its compressed size).
type Object struct {
	ID    ksuid.KSUID `super:"id"`
	Min   super.Value `super:"min"`
	Max   super.Value `super:"max"`
	Count uint64      `super:"count"`
	Size  int64       `super:"size"`
}

func (o Object) IsZero() bool {
	return o.ID == ksuid.Nil
}

func (o Object) String() string {
	return fmt.Sprintf("%s %d record%s in %d data bytes", o.ID, o.Count, plural(int(o.Count)), o.Size)
}

func plural(ordinal int) string {
	if ordinal == 1 {
		return ""
	}
	return "s"
}

func (o *Object) Equal(to *Object) bool {
	return o.ID == to.ID
}

func NewObject() Object {
	return Object{ID: ksuid.New()}
}

func (o Object) Span(order order.Which) *extent.Generic {
	return extent.NewGenericFromOrder(o.Min, o.Max, order)
}

// ObjectPrefix returns a prefix for the various objects that comprise
// a data object so they can all be deleted with the storage engine's
// DeleteByPrefix method.
func (o Object) ObjectPrefix(path *storage.URI) *storage.URI {
	return path.JoinPath(o.ID.String())
}

func (o Object) SequenceURI(path *storage.URI) *storage.URI {
	return SequenceURI(path, o.ID)
}

func SequenceURI(path *storage.URI, id ksuid.KSUID) *storage.URI {
	return path.JoinPath(fmt.Sprintf("%s.bsup", id))
}

func (o Object) SeekIndexURI(path *storage.URI) *storage.URI {
	return SeekIndexURI(path, o.ID)
}

func SeekIndexURI(path *storage.URI, id ksuid.KSUID) *storage.URI {
	return path.JoinPath(fmt.Sprintf("%s-seek.bsup", id))
}

func (o Object) VectorURI(path *storage.URI) *storage.URI {
	return VectorURI(path, o.ID)
}

func VectorURI(path *storage.URI, id ksuid.KSUID) *storage.URI {
	return path.JoinPath(fmt.Sprintf("%s.csup", id))
}

// Remove deletes the row object and its seek index.
// Any 'not found' errors are ignored.
func (o Object) Remove(ctx context.Context, engine storage.Engine, path *storage.URI) error {
	if err := engine.DeleteByPrefix(ctx, o.ObjectPrefix(path)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
