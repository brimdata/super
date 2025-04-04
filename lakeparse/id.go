package lakeparse

import (
	"encoding/hex"
	"fmt"

	"github.com/segmentio/ksuid"
)

func ParseID(s string) (ksuid.KSUID, error) {
	// Check if this is a cut-and-paste from BSUP, which encodes
	// the 20-byte KSUID as a 40 character hex string with 0x prefix.
	var id ksuid.KSUID
	var err error
	if len(s) == 42 && s[0:2] == "0x" {
		var b []byte
		b, err = hex.DecodeString(s[2:])
		if err == nil {
			id, err = ksuid.FromBytes(b)
		}
	} else {
		id, err = ksuid.Parse(s)
	}
	if err != nil {
		return ksuid.Nil, fmt.Errorf("invalid ID: %s", s)
	}
	return id, nil
}

func ParseIDs(ss []string) ([]ksuid.KSUID, error) {
	ids := make([]ksuid.KSUID, 0, len(ss))
	for _, s := range ss {
		id, err := ParseID(s)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
