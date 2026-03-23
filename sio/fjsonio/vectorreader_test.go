package fjsonio

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValueReader(t *testing.T) {
	const s = `{"x":1} {"x":2}`
	r := newValReader(strings.NewReader(s))
	b, err := r.Next()
	require.NoError(t, err)
	fmt.Println("first", string(b))
	b, err = r.Next()
	require.NoError(t, err)
	fmt.Println("next", string(b))
	b, err = r.Next()
	fmt.Println("err", err)
}
