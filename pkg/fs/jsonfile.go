package fs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MarshalJSONFile writes the JSON encoding of v to a file named by filename.
// The file is created atomically. If the file does not exist, Marshal creates
// it with permissions perm.
func MarshalJSONFile(v any, filename string, perm os.FileMode) (err error) {
	return ReplaceFile(filename, perm, func(w io.Writer) error {
		e := json.NewEncoder(w)
		e.SetIndent("", "    ")
		return e.Encode(v)
	})
}

// UnmarshalJSONFile parses JSON-encoded data from the file named by filename
// and stores the result in the value pointed to by v.  Internally, Unmarshal
// uses json.Unmarshal.
func UnmarshalJSONFile(filename string, v any) error {
	data, err := ReadFile(filename)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: unmarshaling error: %w", filename, err)
	}
	return nil
}
