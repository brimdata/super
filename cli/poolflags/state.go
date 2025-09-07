package poolflags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const headFile = ".super_head"

func headPath() string {
	dir := os.Getenv("SUPER_HEAD_DIR")
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if dir == "." {
		dir = ""
	}
	return filepath.Join(dir, headFile)
}

func readHead() (string, error) {
	b, err := os.ReadFile(headPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func WriteHead(pool, branch string) error {
	head := fmt.Sprintf("%s@%s\n", pool, branch)
	err := os.WriteFile(headPath(), []byte(head), 0644)
	if err != nil {
		err = fmt.Errorf("%q: failed to write HEAD: %w", headFile, err)
	}
	return err
}
