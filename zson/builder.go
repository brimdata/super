package zson

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zcode"
	"golang.org/x/text/unicode/norm"
)

func Build(b *zcode.Builder, val Value) (*zed.Value, error) {
	b.Truncate()
	if err := buildValue(b, val); err != nil {
		return nil, err
	}
	it := b.Bytes().Iter()
	return zed.NewValue(val.TypeOf(), it.Next()), nil
}

func buildValue(b *zcode.Builder, val Value) error {
	switch val := val.(type) {
	case *Primitive:
		return BuildPrimitive(b, *val)
	case *Record:
		return buildRecord(b, val)
	case *Array:
		return buildArray(b, val)
	case *Set:
		return buildSet(b, val)
	case *Union:
		return buildUnion(b, val)
	case *Map:
		return buildMap(b, val)
	case *Enum:
		return buildEnum(b, val)
	case *TypeValue:
		return buildTypeValue(b, val)
	case *Error:
		return buildValue(b, val.Value)
	case *Null:
		b.Append(nil)
		return nil
	}
	return fmt.Errorf("unknown ast type: %T", val)
}

func BuildPrimitive(b *zcode.Builder, val Primitive) error {
	switch zed.TypeUnder(val.Type).(type) {
	case *zed.TypeOfUint8, *zed.TypeOfUint16, *zed.TypeOfUint32, *zed.TypeOfUint64:
		v, err := strconv.ParseUint(val.Text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer: %s", val.Text)
		}
		b.Append(zed.EncodeUint(v))
		return nil
	case *zed.TypeOfInt8, *zed.TypeOfInt16, *zed.TypeOfInt32, *zed.TypeOfInt64:
		v, err := strconv.ParseInt(val.Text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer: %s", val.Text)
		}
		b.Append(zed.EncodeInt(v))
		return nil
	case *zed.TypeOfDuration:
		d, err := nano.ParseDuration(val.Text)
		if err != nil {
			return fmt.Errorf("invalid duration: %s", val.Text)
		}
		b.Append(zed.EncodeDuration(d))
		return nil
	case *zed.TypeOfTime:
		t, err := time.Parse(time.RFC3339Nano, val.Text)
		if err != nil {
			return fmt.Errorf("invalid ISO time: %s", val.Text)
		}
		if nano.MaxTs.Time().Sub(t) < 0 {
			return fmt.Errorf("time overflow: %s (max: %s)", val.Text, nano.MaxTs)
		}
		b.Append(zed.EncodeTime(nano.TimeToTs(t)))
		return nil
	case *zed.TypeOfFloat32:
		v, err := strconv.ParseFloat(val.Text, 32)
		if err != nil {
			return fmt.Errorf("invalid floating point: %s", val.Text)
		}
		b.Append(zed.EncodeFloat32(float32(v)))
		return nil
	case *zed.TypeOfFloat64:
		v, err := strconv.ParseFloat(val.Text, 64)
		if err != nil {
			return fmt.Errorf("invalid floating point: %s", val.Text)
		}
		b.Append(zed.EncodeFloat64(v))
		return nil
	case *zed.TypeOfBool:
		var v bool
		if val.Text == "true" {
			v = true
		} else if val.Text != "false" {
			return fmt.Errorf("invalid bool: %s", val.Text)
		}
		b.Append(zed.EncodeBool(v))
		return nil
	case *zed.TypeOfBytes:
		s := val.Text
		if len(s) < 2 || s[0:2] != "0x" {
			return fmt.Errorf("invalid bytes: %s", s)
		}
		var bytes []byte
		if len(s) == 2 {
			// '0x' is an empty byte string (not null byte string)
			bytes = make([]byte, 0, 0)
		} else {
			var err error
			bytes, err = hex.DecodeString(s[2:])
			if err != nil {
				return fmt.Errorf("invalid bytes: %s (%w)", s, err)
			}
		}
		b.Append(zcode.Bytes(bytes))
		return nil
	case *zed.TypeOfString:
		body := zed.EncodeString(val.Text)
		if !utf8.Valid(body) {
			return fmt.Errorf("invalid utf8 string: %q", val.Text)
		}
		b.Append(norm.NFC.Bytes(body))
		return nil
	case *zed.TypeOfIP:
		ip, err := netip.ParseAddr(val.Text)
		if err != nil {
			return err
		}
		b.Append(zed.EncodeIP(ip))
		return nil
	case *zed.TypeOfNet:
		net, err := netip.ParsePrefix(val.Text)
		if err != nil {
			return fmt.Errorf("invalid network: %s (%w)", val.Text, err)
		}
		b.Append(zed.EncodeNet(net.Masked()))
		return nil
	case *zed.TypeOfNull:
		if val.Text != "" {
			return fmt.Errorf("invalid text body of null value: %q", val.Text)
		}
		b.Append(nil)
		return nil
	case *zed.TypeOfType:
		return fmt.Errorf("type values should not be encoded as primitives: %q", val.Text)
	}
	return fmt.Errorf("unknown primitive: %T", val.Type)
}

func buildRecord(b *zcode.Builder, val *Record) error {
	b.BeginContainer()
	for _, v := range val.Fields {
		if err := buildValue(b, v); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}

func buildArray(b *zcode.Builder, array *Array) error {
	b.BeginContainer()
	for _, v := range array.Elements {
		if err := buildValue(b, v); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}

func buildSet(b *zcode.Builder, set *Set) error {
	b.BeginContainer()
	for _, v := range set.Elements {
		if err := buildValue(b, v); err != nil {
			return err
		}
	}
	b.TransformContainer(zed.NormalizeSet)
	b.EndContainer()
	return nil
}

func buildMap(b *zcode.Builder, m *Map) error {
	b.BeginContainer()
	for _, entry := range m.Entries {
		if err := buildValue(b, entry.Key); err != nil {
			return err
		}
		if err := buildValue(b, entry.Value); err != nil {
			return err
		}
	}
	b.TransformContainer(zed.NormalizeMap)
	b.EndContainer()
	return nil
}

func buildUnion(b *zcode.Builder, union *Union) error {
	if tag := union.Tag; tag >= 0 {
		b.BeginContainer()
		b.Append(zed.EncodeInt(int64(tag)))
		if err := buildValue(b, union.Value); err != nil {
			return err
		}
		b.EndContainer()
	} else {
		b.Append(nil)
	}
	return nil
}

func buildEnum(b *zcode.Builder, enum *Enum) error {
	typ, ok := enum.Type.(*zed.TypeEnum)
	if !ok {
		// This shouldn't happen.
		return errors.New("enum value is not of type enum")
	}
	selector := typ.Lookup(enum.Name)
	b.Append(zed.EncodeUint(uint64(selector)))
	return nil
}

func buildTypeValue(b *zcode.Builder, tv *TypeValue) error {
	b.Append(zed.EncodeTypeValue(tv.Value))
	return nil
}
