package csup

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector/vio"

	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
)

var spec = &charm.Spec{
	Name:  "csup",
	Usage: "csup uri",
	Short: "dump CSUP metadata",
	Long: `
csup decodes an input uri and emits the metadata sections in the format desired.`,
	New: New,
}

func init() {
	dev.Spec.Add(spec)
}

type Command struct {
	*dev.Command
	outputFlags outputflags.Flags
	printTypes  bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*dev.Command)}
	c.outputFlags.SetFlags(f)
	f.BoolVar(&c.printTypes, "type", false, "output fused type of file")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("a single file is required")
	}
	uri, err := storage.ParseURI(args[0])
	if err != nil {
		return err
	}
	engine := storage.NewLocalEngine()
	r, err := engine.Get(ctx, uri)
	if err != nil {
		return err
	}
	defer r.Close()
	writer, err := c.outputFlags.Open(ctx, engine)
	if err != nil {
		return err
	}
	if c.printTypes {
		return c.types(r, writer)
	}
	var vals []super.Value
	sctx := super.NewContext()
	marshaler := sup.NewBSUPMarshalerWithContext(sctx)
	for {
		hdr, err := readHeader(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		val, err := marshaler.Marshal(hdr)
		if err != nil {
			return err
		}
		vals = append(vals, val)
		switch hdr.SectionType {
		case csup.SectionObject:
			vals, err = readObject(sctx, marshaler, r, vals)
			if err != nil {
				return err
			}
		case csup.SectionFooter:
			vals, err = readFooter(sctx, marshaler, r, vals)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid CSUP section type: %c", hdr.SectionType)
		}
	}
	err = writer.Push(sbuf.Dematerialize(sctx, sbuf.NewArray(vals)))
	if err2 := writer.Close(); err == nil {
		err = err2
	}
	return err
}

func (c *Command) types(r storage.Reader, w vio.PushCloser) error {
	sctx := super.NewContext()
	typ, err := csup.FusedType(sctx, r)
	if err != nil {
		return err
	}
	val := sctx.LookupTypeValue(typ)
	err = w.Push(sbuf.Dematerialize(sctx, sbuf.NewArray([]super.Value{val})))
	if err2 := w.Close(); err == nil {
		err = err2
	}
	return err
}

func readHeader(r io.Reader) (csup.Header, error) {
	var bytes [csup.HeaderSize]byte
	_, err := io.ReadFull(r, bytes[:])
	if err != nil {
		return csup.Header{}, err
	}
	var hdr csup.Header
	err = hdr.Deserialize(bytes[:])
	return hdr, err
}

func readObject(sctx *super.Context, marshaler *sup.MarshalBSUPContext, r io.Reader, vals []super.Value) ([]super.Value, error) {
	var bytes [csup.DataHeaderSize]byte
	if _, err := io.ReadFull(r, bytes[:]); err != nil {
		return vals, err
	}
	var hdr csup.DataHeader
	if err := hdr.Deserialize(bytes[:]); err != nil {
		return vals, err
	}
	val, err := marshaler.Marshal(hdr)
	if err != nil {
		return vals, err
	}
	vals = append(vals, val)
	metaReader := bsupio.NewReader(sctx, io.LimitReader(r, int64(hdr.MetaSize)))
	for {
		val, err := metaReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if val == nil {
			break
		}
		vals = append(vals, val.Copy())
	}
	if err := metaReader.Close(); err != nil {
		return nil, err
	}
	typedefsReader := bsupio.NewReader(sctx, io.LimitReader(r, int64(hdr.TypeSize)))
	valp, err := typedefsReader.Read()
	if err != nil {
		return nil, err
	}
	if valp.Type() != super.TypeBytes {
		return nil, errors.New("CSUP type section is not a bytes value")
	}
	vals, err = marshalTypeDefs(marshaler, vals, valp.Bytes())
	if err != nil {
		return nil, err
	}
	valp, err = typedefsReader.Read()
	if err != nil && err != io.EOF {
		return nil, err
	}
	if valp != nil {
		return nil, errors.New("CSUP type section has more than one value")
	}
	buf, err := io.ReadAll(io.LimitReader(r, int64(hdr.DataSize)))
	if err == nil && len(buf) != int(hdr.DataSize) {
		err = fmt.Errorf("truncated CSUP data: data section %d but read only %d", hdr.DataSize, len(buf))
	}
	return vals, err
}

func readFooter(sctx *super.Context, marshaler *sup.MarshalBSUPContext, r io.Reader, vals []super.Value) ([]super.Value, error) {
	var bytes [csup.FooterSize]byte
	if _, err := io.ReadFull(r, bytes[:]); err != nil {
		return vals, err
	}
	var f csup.Footer
	f.Deserialize(bytes[:])
	val, err := marshaler.Marshal(f)
	if err != nil {
		return vals, err
	}
	vals = append(vals, val)
	typeBytes := make([]byte, f.MetaSize)
	if _, err := io.ReadFull(r, typeBytes); err != nil {
		return vals, err
	}
	typ, err := sctx.LookupByValue(typeBytes)
	if err != nil {
		return vals, err
	}
	vals = append(vals, sctx.LookupTypeValue(typ))
	var trailerBytes [csup.TrailerSize]byte
	if _, err := io.ReadFull(r, trailerBytes[:]); err != nil {
		return vals, err
	}
	var t csup.Trailer
	if err := t.Deserialize(trailerBytes[:]); err != nil {
		return vals, err
	}
	if val, err = marshaler.Marshal(t); err != nil {
		return vals, err
	}
	vals = append(vals, val)
	return vals, nil
}

func marshalTypeDefs(marshaler *sup.MarshalBSUPContext, vals []super.Value, bytes []byte) ([]super.Value, error) {
	id := uint32(super.IDTypeComplex)
	for len(bytes) > 0 {
		var desc any
		bytes, desc = decodeTypeDef(id, bytes)
		if desc != nil {
			val, err := marshaler.Marshal(desc)
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
		id++
	}
	return vals, nil
}

func DecodeTypeDefs(bytes []byte, offset int) ([]any, error) {
	id := uint32(offset + super.IDTypeComplex)
	var out []any
	for len(bytes) > 0 {
		var desc any
		bytes, desc = decodeTypeDef(id, bytes)
		if desc != nil {
			out = append(out, desc)
		}
		id++
	}
	return out, nil
}

func decodeTypeDef(slot uint32, bytes []byte) ([]byte, any) {
	var out any
	typedef := bytes[0]
	bytes = bytes[1:]
	var n int
	var name string
	var id uint32
	switch typedef {
	case super.TypeDefNamed:
		name, bytes = super.DecodeName(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefNamed", "at name field")
		}
		id, bytes = super.DecodeFixedID(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefNamed", "at ID field")
		}
		out = &struct {
			Kind string
			Slot uint32
			Name string
			ID   uint32
		}{
			Kind: "TypeDefNamed",
			Slot: slot,
			Name: name,
			ID:   id,
		}
	case super.TypeDefRecord:
		type Field struct {
			Name string
			ID   uint32
		}
		n, bytes = super.DecodeLength(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefRecord", "at length field")
		}
		var fields []Field
		for range n {
			name, bytes = super.DecodeName(bytes)
			if bytes == nil {
				return nil, errInfo(slot, "TypeDefRecord", "at field name field")
			}
			id, bytes = super.DecodeID(bytes)
			if bytes == nil {
				return nil, errInfo(slot, "TypeDefRecord", "at field ID field")
			}
			fields = append(fields, Field{name, id})
		}
		out = &struct {
			Kind   string
			Slot   uint32
			Fields []Field
		}{
			Kind:   "TypeDefRecord",
			Slot:   slot,
			Fields: fields,
		}
	case super.TypeDefArray:
		bytes, out = wrapped(bytes, "TypeDefArray", slot)
	case super.TypeDefSet:
		bytes, out = wrapped(bytes, "TypeDefSet", slot)
	case super.TypeDefError:
		bytes, out = wrapped(bytes, "TypeDefError", slot)
	case super.TypeDefFusion:
		bytes, out = wrapped(bytes, "TypeDefFusion", slot)
	case super.TypeDefMap:
		var keyID, valID uint32
		keyID, bytes = super.DecodeID(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefMap", "at key ID")
		}
		valID, bytes = super.DecodeID(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefMap", "at value ID")
		}
		out = &struct {
			Kind  string
			Slot  uint32
			KeyID uint32
			ValID uint32
		}{
			Kind:  "TypeDefMap",
			Slot:  slot,
			KeyID: keyID,
			ValID: valID,
		}
	case super.TypeDefUnion:
		n, bytes = super.DecodeLength(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefUnion", "at length field")
		}
		if n > super.MaxUnionTypes {
			return nil, errInfo(slot, "TypeDefUnion", "at length field (size exceed)")
		}
		var ids []uint32
		for range n {
			id, bytes = super.DecodeID(bytes)
			if bytes == nil {
				return nil, errInfo(slot, "TypeDefUnion", "in type ID list")
			}
			ids = append(ids, id)
		}
		out = &struct {
			Kind string
			Slot uint32
			IDs  []uint32
		}{
			Kind: "TypeDefUnion",
			Slot: slot,
			IDs:  ids,
		}
	case super.TypeDefEnum:
		n, bytes = super.DecodeLength(bytes)
		if bytes == nil {
			return nil, errInfo(slot, "TypeDefEnum", "at length field")
		}
		if n > super.MaxEnumSymbols {
			return nil, errInfo(slot, "TypeDefEnum", "at length field (size exceed)")
		}
		var names []string
		for range n {
			name, bytes = super.DecodeName(bytes)
			if bytes == nil {
				return nil, errInfo(slot, "TypeDefEnum", "at enum symbol")
			}
			names = append(names, name)
		}
		out = &struct {
			Kind  string
			Slot  uint32
			Names []string
		}{
			Kind:  "TypeDefEnum",
			Slot:  slot,
			Names: names,
		}
	default:
		out = &struct {
			Kind string
			Slot uint32
			Code int
		}{
			Kind: "Bad TypeDef code",
			Slot: slot,
			Code: int(typedef),
		}
		bytes = nil
	}
	return bytes, out
}

func wrapped(bytes []byte, kind string, slot uint32) ([]byte, any) {
	var id uint32
	id, bytes = super.DecodeID(bytes)
	if bytes == nil {
		return nil, errInfo(slot, kind, "at ID field")
	}
	return bytes, &struct {
		Kind string
		Slot uint32
		ID   uint32
	}{
		Kind: kind,
		Slot: slot,
		ID:   id,
	}
}

func errInfo(slot uint32, typedef, message string) any {
	return &struct {
		Kind    string
		Slot    uint32
		TypeDef string
		Where   string
	}{
		Kind:    "Decode Error",
		Slot:    slot,
		TypeDef: typedef,
		Where:   message,
	}
}
