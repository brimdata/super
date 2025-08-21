package fuzz

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/netip"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/pkg/storage/mock"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
	"github.com/stretchr/testify/require"
	"github.com/x448/float16"
	"go.uber.org/mock/gomock"
)

func ReadBSUP(bs []byte) ([]super.Value, error) {
	bytesReader := bytes.NewReader(bs)
	context := super.NewContext()
	reader := bsupio.NewReader(context, bytesReader)
	defer reader.Close()
	var a sbuf.Array
	err := sio.Copy(&a, reader)
	if err != nil {
		return nil, err
	}
	return a.Values(), nil
}

func ReadCSUP(bs []byte, fields []field.Path) ([]super.Value, error) {
	bytesReader := bytes.NewReader(bs)
	context := super.NewContext()
	reader, err := csupio.NewReader(context, bytesReader, fields)
	if err != nil {
		return nil, err
	}
	var a sbuf.Array
	err = sio.Copy(&a, reader)
	if err != nil {
		return nil, err
	}
	return a.Values(), nil
}

func WriteBSUP(t testing.TB, valuesIn []super.Value, buf *bytes.Buffer) {
	writer := bsupio.NewWriter(sio.NopCloser(buf))
	require.NoError(t, sio.Copy(writer, sbuf.NewArray(valuesIn)))
	require.NoError(t, writer.Close())
}

func WriteCSUP(t testing.TB, valuesIn []super.Value, buf *bytes.Buffer) {
	writer := csupio.NewWriter(sio.NopCloser(buf))
	require.NoError(t, sio.Copy(writer, sbuf.NewArray(valuesIn)))
	require.NoError(t, writer.Close())
}

func RunQueryBSUP(t testing.TB, buf *bytes.Buffer, querySource string) []super.Value {
	sctx := super.NewContext()
	readers := []sio.Reader{bsupio.NewReader(sctx, buf)}
	defer sio.CloseReaders(readers)
	return RunQuery(t, sctx, readers, querySource, func(_ demand.Demand) {})
}

func RunQueryCSUP(t testing.TB, buf *bytes.Buffer, querySource string) []super.Value {
	sctx := super.NewContext()
	reader, err := csupio.NewReader(sctx, bytes.NewReader(buf.Bytes()), nil)
	require.NoError(t, err)
	readers := []sio.Reader{reader}
	defer sio.CloseReaders(readers)
	return RunQuery(t, sctx, readers, querySource, func(_ demand.Demand) {})
}

func RunQuery(t testing.TB, sctx *super.Context, readers []sio.Reader, querySource string, useDemand func(demandIn demand.Demand)) []super.Value {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Compile query
	engine := mock.NewMockEngine(gomock.NewController(t))
	comp := compiler.NewCompiler(engine)
	ast, err := parser.ParseQuery(querySource)
	if err != nil {
		t.Skipf("%v", err)
	}
	query, err := runtime.CompileQuery(ctx, sctx, comp, ast, readers)
	if err != nil {
		t.Skipf("%v", err)
	}
	defer query.Pull(true)

	// Infer demand
	// TODO This is a hack and should be replaced by a cleaner interface in CompileQuery.
	env := exec.NewEnvironment(engine, nil)
	dag, err := semantic.Analyze(ctx, ast, env, true)
	if err != nil {
		t.Skipf("%v", err)
	}
	if len(dag) > 0 {
		demand := demand.Union(optimizer.DemandForSeq(dag, demand.All())...)
		useDemand(demand)
	}

	// Run query
	var valuesOut []super.Value
	for {
		batch, err := query.Pull(false)
		require.NoError(t, err)
		if batch == nil {
			break
		}
		for _, value := range batch.Values() {
			valuesOut = append(valuesOut, value.Copy())
		}
		batch.Unref()
	}

	return valuesOut
}

func CompareValues(t testing.TB, valuesExpected []super.Value, valuesActual []super.Value) {
	t.Logf("comparing: len(expected)=%v vs len(actual)=%v", len(valuesExpected), len(valuesActual))
	for i := range valuesExpected {
		if i >= len(valuesActual) {
			t.Errorf("missing value: expected[%v].Bytes()=%v", i, valuesExpected[i].Bytes())
			t.Errorf("missing value: expected[%v]=%v", i, sup.String(&valuesExpected[i]))
			continue
		}
		valueExpected := valuesExpected[i]
		valueActual := valuesActual[i]
		t.Logf("comparing: expected[%v]=%v vs actual[%v]=%v", i, sup.String(&valueExpected), i, sup.String(&valueActual))
		if !bytes.Equal(super.EncodeTypeValue(valueExpected.Type()), super.EncodeTypeValue(valueActual.Type())) {
			t.Errorf("values have different types: %v vs %v", valueExpected.Type(), valueActual.Type())
		}
		if !bytes.Equal(valueExpected.Bytes(), valueActual.Bytes()) {
			t.Errorf("values have different BSUP bytes: %v vs %v", valueExpected.Bytes(), valueActual.Bytes())
		}
	}
	for i := range valuesActual[len(valuesExpected):] {
		t.Errorf("extra value: actual[%v].Bytes()=%v", i, valuesActual[i].Bytes())
		t.Errorf("extra value: actual[%v]=%v", i, sup.String(&valuesActual[i]))
	}
}

func GenValues(b *bytes.Reader, context *super.Context, types []super.Type) []super.Value {
	var values []super.Value
	var builder zcode.Builder
	for GenByte(b) != 0 {
		typ := types[int(GenByte(b))%len(types)]
		builder.Reset()
		GenValue(b, context, typ, &builder)
		values = append(values, super.NewValue(typ, builder.Bytes().Body()))
	}
	return values
}

func GenValue(b *bytes.Reader, context *super.Context, typ super.Type, builder *zcode.Builder) {
	if GenByte(b) == 0 {
		builder.Append(nil)
		return
	}
	switch typ {
	case super.TypeUint8:
		builder.Append(super.EncodeUint(uint64(GenByte(b))))
	case super.TypeUint16:
		builder.Append(super.EncodeUint(uint64(binary.LittleEndian.Uint16(GenBytes(b, 2)))))
	case super.TypeUint32:
		builder.Append(super.EncodeUint(uint64(binary.LittleEndian.Uint32(GenBytes(b, 4)))))
	case super.TypeUint64:
		builder.Append(super.EncodeUint(uint64(binary.LittleEndian.Uint64(GenBytes(b, 8)))))
	case super.TypeInt8:
		builder.Append(super.EncodeInt(int64(GenByte(b))))
	case super.TypeInt16:
		builder.Append(super.EncodeInt(int64(binary.LittleEndian.Uint16(GenBytes(b, 2)))))
	case super.TypeInt32:
		builder.Append(super.EncodeInt(int64(binary.LittleEndian.Uint32(GenBytes(b, 4)))))
	case super.TypeInt64:
		builder.Append(super.EncodeInt(int64(binary.LittleEndian.Uint64(GenBytes(b, 8)))))
	case super.TypeDuration:
		builder.Append(super.EncodeDuration(nano.Duration(int64(binary.LittleEndian.Uint64(GenBytes(b, 8))))))
	case super.TypeTime:
		builder.Append(super.EncodeTime(nano.Ts(int64(binary.LittleEndian.Uint64(GenBytes(b, 8))))))
	case super.TypeFloat16:
		builder.Append(super.EncodeFloat16(float32(float16.Frombits(binary.LittleEndian.Uint16(GenBytes(b, 4))))))
	case super.TypeFloat32:
		builder.Append(super.EncodeFloat32(math.Float32frombits(binary.LittleEndian.Uint32(GenBytes(b, 4)))))
	case super.TypeFloat64:
		builder.Append(super.EncodeFloat64(math.Float64frombits(binary.LittleEndian.Uint64(GenBytes(b, 8)))))
	case super.TypeBool:
		builder.Append(super.EncodeBool(GenByte(b) > 0))
	case super.TypeBytes:
		builder.Append(super.EncodeBytes(GenBytes(b, int(GenByte(b)))))
	case super.TypeString:
		builder.Append(super.EncodeString(string(GenBytes(b, int(GenByte(b))))))
	case super.TypeIP:
		builder.Append(super.EncodeIP(netip.AddrFrom16([16]byte(GenBytes(b, 16)))))
	case super.TypeNet:
		ip := netip.AddrFrom16([16]byte(GenBytes(b, 16)))
		numBits := int(GenByte(b)) % ip.BitLen()
		net, err := ip.Prefix(numBits)
		if err != nil {
			// Should be unreachable.
			panic(err)
		}
		builder.Append(super.EncodeNet(net))
	case super.TypeType:
		typ := GenType(b, context, 3)
		builder.Append(super.EncodeTypeValue(typ))
	case super.TypeNull:
		builder.Append(nil)
	default:
		switch typ := typ.(type) {
		case *super.TypeRecord:
			builder.BeginContainer()
			for _, field := range typ.Fields {
				GenValue(b, context, field.Type, builder)
			}
			builder.EndContainer()
		case *super.TypeArray:
			builder.BeginContainer()
			for GenByte(b) != 0 {
				GenValue(b, context, typ.Type, builder)
			}
			builder.EndContainer()
		case *super.TypeMap:
			builder.BeginContainer()
			for GenByte(b) != 0 {
				GenValue(b, context, typ.KeyType, builder)
				GenValue(b, context, typ.ValType, builder)
			}
			builder.TransformContainer(super.NormalizeMap)
			builder.EndContainer()
		case *super.TypeSet:
			builder.BeginContainer()
			for GenByte(b) != 0 {
				GenValue(b, context, typ.Type, builder)
			}
			builder.TransformContainer(super.NormalizeSet)
			builder.EndContainer()
		case *super.TypeUnion:
			tag := binary.LittleEndian.Uint64(GenBytes(b, 8)) % uint64(len(typ.Types))
			builder.BeginContainer()
			builder.Append(super.EncodeInt(int64(tag)))
			GenValue(b, context, typ.Types[tag], builder)
			builder.EndContainer()
		default:
			panic("Unreachable")
		}
	}
}

func GenTypes(b *bytes.Reader, context *super.Context, depth int) []super.Type {
	var types []super.Type
	for len(types) == 0 || GenByte(b) != 0 {
		types = append(types, GenType(b, context, depth))
	}
	return types
}

func GenType(b *bytes.Reader, context *super.Context, depth int) super.Type {
	if depth < 0 || GenByte(b)%2 == 0 {
		switch GenByte(b) % 19 {
		case 0:
			return super.TypeUint8
		case 1:
			return super.TypeUint16
		case 2:
			return super.TypeUint32
		case 3:
			return super.TypeUint64
		case 4:
			return super.TypeInt8
		case 5:
			return super.TypeInt16
		case 6:
			return super.TypeInt32
		case 7:
			return super.TypeInt64
		case 8:
			return super.TypeDuration
		case 9:
			return super.TypeTime
		case 10:
			return super.TypeFloat16
		case 11:
			return super.TypeFloat32
		case 12:
			return super.TypeBool
		case 13:
			return super.TypeBytes
		case 14:
			return super.TypeString
		case 15:
			return super.TypeIP
		case 16:
			return super.TypeNet
		case 17:
			return super.TypeType
		case 18:
			return super.TypeNull
		default:
			panic("Unreachable")
		}
	} else {
		depth--
		switch GenByte(b) % 5 {
		case 0:
			fieldTypes := GenTypes(b, context, depth)
			fields := make([]super.Field, len(fieldTypes))
			for i, fieldType := range fieldTypes {
				fields[i] = super.Field{
					Name: fmt.Sprintf("f%d", i),
					Type: fieldType,
				}
			}
			typ, err := context.LookupTypeRecord(fields)
			if err != nil {
				panic(err)
			}
			return typ
		case 1:
			elem := GenType(b, context, depth)
			return context.LookupTypeArray(elem)
		case 2:
			key := GenType(b, context, depth)
			value := GenType(b, context, depth)
			return context.LookupTypeMap(key, value)
		case 3:
			elem := GenType(b, context, depth)
			return context.LookupTypeSet(elem)
		case 4:
			types := GenTypes(b, context, depth)
			// TODO There are some weird corners around unions that contain null or duplicate types eg
			// csup_test.go:107: comparing: in[0]=null((null,null)) vs out[0]=null((null,null))
			// csup_test.go:112: values have different BSUP bytes: [1 0] vs [2 2 0]
			var unionTypes []super.Type
			for _, typ := range types {
				skip := false
				if typ == super.TypeNull {
					skip = true
				}
				for _, unionType := range unionTypes {
					if typ == unionType {
						skip = true
					}
				}
				if !skip {
					unionTypes = append(unionTypes, typ)
				}
			}
			if len(unionTypes) == 0 {
				return super.TypeNull
			}
			return context.LookupTypeUnion(unionTypes)
		default:
			panic("Unreachable")
		}
	}
}

func GenByte(b *bytes.Reader) byte {
	// If we're out of bytes, return 0.
	byte, err := b.ReadByte()
	if err != nil && !errors.Is(err, io.EOF) {
		panic(err)
	}
	return byte
}

func GenBytes(b *bytes.Reader, n int) []byte {
	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = GenByte(b)
	}
	return bytes
}

func GenAscii(b *bytes.Reader) string {
	var bytes []byte
	for {
		byte := GenByte(b)
		if byte == 0 {
			break
		}
		bytes = append(bytes, byte)
	}
	return string(bytes)
}
