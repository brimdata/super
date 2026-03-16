package jsonio

import (
	"testing"

	"github.com/bytedance/sonic/ast"

	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	b := NewBuilder()
	err := ast.Preorder(`{"x":1,"y":1,"z":"foo"}`, b, nil)
	require.NoError(t, err)
	err = ast.Preorder(`{"x":2}`, b, nil)
	require.NoError(t, err)
	err = ast.Preorder(`{"y":3}`, b, nil)
	require.NoError(t, err)
	err = ast.Preorder(`{"x":4,"z":"bar"}`, b, nil)
	require.NoError(t, err)
	Materialize(b)
}
