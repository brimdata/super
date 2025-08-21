package dbid

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseID(t *testing.T) {
	id, err := ParseID("0x0123456789012345678901234567890123456789")
	assert.NoError(t, err)
	assert.Equal(t, "0A42ooXOWFkGit78ZjPVLpDkRgn", id.String())

	// _ isn't a valid hexadecimal digit.
	_, err = ParseID("0x012345678901234567890123456789012345678_")
	assert.EqualError(t, err, "invalid ID: 0x012345678901234567890123456789012345678_")

	id, err = ParseID("0A42ooXOWFkGit78ZjPVLpDkRgn")
	assert.NoError(t, err)
	assert.Equal(t, "0A42ooXOWFkGit78ZjPVLpDkRgn", id.String())

	// Too long.
	_, err = ParseID("0A42ooXOWFkGit78ZjPVLpDkRgnn")
	assert.EqualError(t, err, "invalid ID: 0A42ooXOWFkGit78ZjPVLpDkRgnn")

	// Too short.
	_, err = ParseID("0A42ooXOWFkGit78ZjPVLpDkRg")
	assert.EqualError(t, err, "invalid ID: 0A42ooXOWFkGit78ZjPVLpDkRg")
}
