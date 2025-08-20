package zeekio

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
)

var ErrIncompatibleZeekType = errors.New("type cannot be represented in zeek format")

func superTypeToZeek(typ super.Type) (string, error) {
	switch typ := typ.(type) {
	case *super.TypeArray:
		inner, err := superTypeToZeek(typ.Type)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("vector[%s]", inner), nil
	case *super.TypeSet:
		inner, err := superTypeToZeek(typ.Type)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("set[%s]", inner), nil
	case *super.TypeOfUint8, *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfUint16, *super.TypeOfUint32:
		return "int", nil
	case *super.TypeOfUint64:
		return "count", nil
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return "double", nil
	case *super.TypeOfIP:
		return "addr", nil
	case *super.TypeOfNet:
		return "subnet", nil
	case *super.TypeOfDuration:
		return "interval", nil
	case *super.TypeNamed:
		if typ.Name == "zenum" {
			return "enum", nil
		}
		if typ.Name == "port" {
			return "port", nil
		}
		return superTypeToZeek(typ.Type)
	case *super.TypeOfBool, *super.TypeOfString, *super.TypeOfTime:
		return super.PrimitiveName(typ), nil
	default:
		return "", fmt.Errorf("type %s: %w", typ, ErrIncompatibleZeekType)
	}
}
