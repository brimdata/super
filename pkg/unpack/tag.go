package unpack

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	tagJSON   = "json"
	tagUnpack = "unpack"
	tagSep    = ","
)

var ErrSkip = errors.New("unpack skip tag can only appear once")

func parseTag(which string, f reflect.StructField) (string, bool, []string) {
	tag, ok := f.Tag.Lookup(which)
	if !ok {
		return "", false, nil
	}
	if tag == "" {
		return "", true, nil
	}
	elems := strings.Split(tag, tagSep)
	var opts []string
	if len(elems) > 1 {
		opts = elems[1:]
	}
	return elems[0], true, opts
}

func jsonFieldName(f reflect.StructField) (string, bool) {
	s, ok, _ := parseTag(tagJSON, f)
	return s, ok
}

// structToRule looks through the tags in a struct to (1) make sure there
// are no duplicate tags since package json doesn't do this and it results
// in hard-to-debug behaviors, and (2) it finds an "unpack" tag which indicates
// which field of the struct to use as the key and what value to match for
// unmarshling into this concrete type.  If there is not exactly one unpack
// tag, then an error is returned.
// unpack="key,skip" unpack="key", unpack="", or unpack="skip"
func structToUnpackRule(typ reflect.Type) (string, string, bool, error) {
	if typ.Kind() != reflect.Struct {
		return "", "", false, fmt.Errorf("cannot unpack into non-struct type '%s'", typ)
	}
	names := make(map[string]struct{})
	var unpackKey string
	var unpackVal string
	var unpackSkip bool
	for k := range typ.NumField() {
		field := typ.Field(k)
		jsonField, jsonOk, _ := parseTag(tagJSON, field)
		if jsonOk {
			if _, ok := names[jsonField]; ok {
				return "", "", false, fmt.Errorf("JSON field tag '%s' in struct type '%s' not unique", jsonField, typ.Name())
			}
			names[jsonField] = struct{}{}
		}
		unpackOpt, unpackOk, opts := parseTag(tagUnpack, field)
		if !unpackOk {
			continue
		}
		if jsonField == "-" {
			return "", "", false, fmt.Errorf("unpack: cannot unpack field '%s' of struct type '%s' with JSON tag '-'", field.Name, typ.Name())
		}
		if len(opts) > 1 {
			return "", "", false, fmt.Errorf("unpack: too many tag options in field '%s' of struct type '%s'", field.Name, typ.Name())
		}
		var skip bool
		if len(opts) == 1 {
			if opts[0] != "skip" {
				return "", "", false, fmt.Errorf("unpack: second tag option in field '%s' of struct type '%s' may only be 'skip'", field.Name, typ.Name())
			}
			skip = true
		}
		if skip {
			if unpackSkip {
				return "", "", false, ErrSkip
			}
			unpackSkip = true
		}
		if unpackKey != "" {
			return "", "", false, fmt.Errorf("unpack key appears twice in field '%s' of struct type '%s' may only be 'skip'", field.Name, typ.Name())
		}
		if jsonField == "" {
			jsonField = field.Name
		}
		if unpackOpt == "" {
			unpackOpt = typ.Name()
			if unpackOpt == "" {
				return "", "", false, fmt.Errorf("unpack tag missing value struct type is nameless in field '%s' of struct '%s' may only be 'skip'", field.Name, typ.String())
			}
		}
		unpackKey = jsonField
		unpackVal = unpackOpt
	}
	return unpackKey, unpackVal, unpackSkip, nil
}
