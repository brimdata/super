// Package unpack provides a means to unmarshal Go values that have embedded
// interface values.  Different concrete implementations of any interface value
// are properly decoded using an `unpack` struct tag to indicate a JSON field
// identifying the desired concrete type.  To do so, a client of
// unpack registers each potential type in a Reflector, which binds a field
// with a particular value to that type.
package unpack

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
)

var zero reflect.Value

type Reflector map[string]map[string]reflect.Type

func New(templates ...interface{}) Reflector {
	r := make(Reflector)
	for _, t := range templates {
		r.Add(t)
	}
	return r
}

func (r Reflector) mixIn(other Reflector) Reflector {
	for k, v := range other {
		r[k] = v
	}
	return r
}

func (r Reflector) Add(template interface{}) Reflector {
	return r.AddAs(template, "")
}

// AddAs is like Add but as overrides any name stored under the "unpack" key in
// template's field tags.
func (r Reflector) AddAs(template interface{}, as string) Reflector {
	if another, ok := template.(Reflector); ok {
		return r.mixIn(another)
	}
	typ := reflect.TypeOf(template)
	unpackKey, unpackVal, skip, err := structToUnpackRule(typ)
	if err != nil {
		panic(err)
	}
	if as != "" {
		unpackVal = as
	}
	if unpackKey == "" {
		panic(fmt.Sprintf("unpack tag not found for Go type %q", typ.String()))
	}
	types, ok := r[unpackKey]
	if !ok {
		types = make(map[string]reflect.Type)
		r[unpackKey] = types
	}
	if _, ok := types[unpackVal]; ok {
		panic(fmt.Sprintf("unpack binding for JSON field %q and Go type %q already exists", unpackKey, unpackVal))
	}
	if skip {
		types[unpackVal] = nil
	} else {
		types[unpackVal] = typ
	}
	return r
}

func (r Reflector) UnmarshalString(s string) (interface{}, error) {
	return r.Unmarshal([]byte(s))
}

func (r Reflector) Unmarshal(b []byte) (interface{}, error) {
	var val interface{}
	if err := json.Unmarshal(b, &val); err != nil {
		return nil, fmt.Errorf("unpacker error parsing JSON: %w", err)
	}
	skeleton, err := walk(val, r.unpack)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &skeleton); err != nil {
		return nil, err
	}
	return skeleton, err
}

// XXX We added method in branch seq but we realized we should replace
// Unmarshal with this logic, which is consistent with json.Unmarshal.
// See issue #4557.
func (r Reflector) UnmarshalInto(b []byte, result interface{}) error {
	var from interface{}
	if err := json.Unmarshal(b, &from); err != nil {
		return fmt.Errorf("unpacker error parsing JSON: %w", err)
	}
	if err := r.unpackVal(reflect.ValueOf(result), from); err != nil {
		return err
	}
	return json.Unmarshal(b, &result)
}

func (r Reflector) UnmarshalObject(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return r.Unmarshal(b)
}

func (r Reflector) lookup(object map[string]interface{}) (reflect.Value, error) {
	var hits int
	for key, val := range object {
		types := r[key]
		if types == nil {
			continue
		}
		unpackVal, ok := val.(string)
		if !ok {
			return zero, fmt.Errorf("unpack key in JSON field %q is not a string: '%T'", key, val)
		}
		hits++
		if template, ok := types[unpackVal]; ok {
			if template == nil {
				// skip
				return zero, nil
			}
			return reflect.New(template), nil
		}
	}
	// If we hit a key but it didn't have any matching rule (even to skip),
	// then we raise an error.
	if hits > 0 {
		return zero, fmt.Errorf("unpack: JSON object found with candidate key(s) having no template match\n%s", stringify(object))
	}
	return zero, nil
}

func stringify(val interface{}) string {
	b, err := json.Marshal(val)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (r Reflector) unpack(from interface{}) (interface{}, error) {
	object, ok := from.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	toVal, err := r.lookup(object)
	if toVal == zero || err != nil {
		return nil, err
	}
	if err := r.unpackStruct(toVal.Elem(), object); err != nil {
		return nil, err
	}
	return toVal.Interface(), nil
}

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

func (r Reflector) unpackVal(toVal reflect.Value, from interface{}) error {
	if from == nil {
		return nil
	}
	// If this value implements encoding.TextUnmarshaler and the JSON
	// value is a string, then just return and let the unmarshaler handle
	// things thus avoiding a type mismatch below.
	if toVal.Type().NumMethod() != 0 && toVal.CanInterface() {
		if _, ok := from.(string); ok {
			if typ := toVal.Type(); typ.Implements(textUnmarshalerType) ||
				reflect.PtrTo(typ).Implements(textUnmarshalerType) {
				return nil
			}
		}
	}
	switch toVal.Kind() {
	case reflect.Interface:
		// Here is the magical move.  For all interface values, we need to
		// be able to find a concrete implementation that package json
		// can unmarshal into.  So we call unpack recursively here and require
		// that this finds such a concrete value.  We install a zero-valued instance
		// of this concrete value and let package json fill it in on the final pass.
		child, err := r.unpack(from)
		if err != nil {
			return err
		}
		if child == nil {
			child = from
		}
		if err := assign(toVal, reflect.ValueOf(child)); err != nil {
			return err
		}
	case reflect.Ptr:
		var elem reflect.Value
		if toVal.IsNil() {
			elem = reflect.New(toVal.Type().Elem())
			toVal.Set(elem)
		} else {
			elem = toVal.Elem()
		}
		return r.unpackVal(elem, from)
	case reflect.Struct:
		if object, ok := from.(map[string]interface{}); ok {
			return r.unpackStruct(toVal, object)
		}
		return typeErr(toVal, from)
	// For arrays and slices, we always try to unpack the elements just in case
	// there are interface values somewhere below in the hierarchy that need
	// to be handled.  If not and everything is static, this doesn't hurt as
	// package json would have done the same work anyway.
	case reflect.Slice:
		elems, ok := from.([]interface{})
		if !ok {
			return typeErr(toVal, elems)
		}
		toVal.Set(reflect.MakeSlice(toVal.Type(), len(elems), len(elems)))
		return r.unpackElems(toVal, elems)
	case reflect.Array:
		elems, ok := from.([]interface{})
		if !ok {
			return typeErr(toVal, from)
		}
		toVal.Set(reflect.New(toVal.Type()).Elem())
		return r.unpackElems(toVal, elems)
	}
	return nil
}

func (r Reflector) unpackElems(toVal reflect.Value, from []interface{}) error {
	for k, elem := range from {
		if err := r.unpackVal(toVal.Index(k), elem); err != nil {
			return err
		}
	}
	return nil
}

func (r Reflector) unpackStruct(toVal reflect.Value, from map[string]interface{}) error {
	// Create a struct of the desired concrete type then for each field of
	// the interface type, copy the object from the map input argment.
	// The final pass of the JSON decoder will fill in everything else since
	// all we can about is getting the interfaces right.
	structType := toVal.Type()
	for i := 0; i < structType.NumField(); i++ {
		fieldName, ok := jsonFieldName(structType.Field(i))
		if !ok {
			// No JSON tag on this field.
			continue
		}
		o, ok := from[fieldName]
		if !ok {
			// Skip over values in the conrete struct
			// that do not have keys in the json leaving that
			// field as a zero value, just as the Golang JSON
			// decoder does.
			continue
		}
		if err := r.unpackVal(toVal.Field(i), o); err != nil {
			return fmt.Errorf("JSON field %q in Go struct type %q: %w", fieldName, toVal.Type(), err)
		}
	}
	return nil
}

func walk(val interface{}, pre func(interface{}) (interface{}, error)) (interface{}, error) {
	if done, err := pre(val); done != nil || err != nil {
		return done, err
	}
	switch val := val.(type) {
	case map[string]interface{}:
		for k, v := range val {
			child, err := walk(v, pre)
			if err != nil {
				return nil, err
			}
			val[k] = child
		}
	case []interface{}:
		for k, v := range val {
			child, err := walk(v, pre)
			if err != nil {
				return nil, err
			}
			val[k] = child
		}
	}
	return val, nil
}

func assign(dst reflect.Value, src reflect.Value) error {
	if !src.Type().AssignableTo(dst.Type()) {
		return fmt.Errorf("value of type %q not assignable to type %q", src.Type(), dst.Type())
	}
	dst.Set(src)
	return nil
}

func typeErr(toVal reflect.Value, from interface{}) error {
	return fmt.Errorf("unpacking into type %s: incompatible JSON: %s", toVal.Type(), stringify(from))
}
