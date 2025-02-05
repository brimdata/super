package skim_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/brimdata/super/pkg/skim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ReadSize    = 64 * 1024
	MaxLineSize = 50 * 1024 * 1024
)

func makeLinesOfSize(size int) [][]byte {
	var lines [][]byte
	var count int
	for {
		l := fmt.Sprintf("I love zeek data!\tLorem ipsum dolor sit amet, consectetur adipiscing elit. Quisque tincidunt turpis nunc, viverra viverra orci porta nec. Fusce imperdiet felis non bibendum aliquam. In hac habitasse platea dictumst. Aenean id fermentum mi, at sagittis lectus. Integer vel tempus neque, ac accumsan urna. Curabitur et aliquam ligula. Fusce tempus fringilla orci, a vestibulum elit. Sed accumsan vehicula lorem, et auctor est sagittis eget. Proin ut tellus non eros iaculis accumsan eget ut ipsum. Phasellus vulputate mauris sit amet semper eleifend. Vestibulum lacus nisl, laoreet eu nulla a, euismod pulvinar turpis. Maecenas vel volutpat odio. Morbi finibus, dolor sed ultricies sollicitudin, augue ex accumsan nisl, eget feugiat nunc ipsum id massa. Nulla rutrum augue ut elit ullamcorper, vitae euismod augue pharetra. Sed in enim nec eros tincidunt euismod. Donec ullamcorper finibus viverra. Morbi eros tellus, suscipit sed nibh eu, pharetra eleifend nibh\t%d", len(lines))
		count += len(l)
		lines = append(lines, []byte(l))
		if count > size {
			break
		}
	}
	return lines
}

func TestSkim(t *testing.T) {
	expected := makeLinesOfSize(MaxLineSize)
	data := bytes.Join(expected, []byte("\n"))
	buf := make([]byte, ReadSize)
	scanner := skim.NewScanner(bytes.NewReader(data), buf, MaxLineSize)
	var lines [][]byte
	var i int
	for {
		line, err := scanner.ScanLine()
		if err != nil {
			t.Fatal(err)
		}
		if line == nil {
			break
		}
		require.Equal(t, string(expected[i]), string(bytes.TrimSpace(line)))
		lines = append(lines, line)
		i++
	}
	require.Len(t, lines, len(expected))
}

func TestSkimNoNewLine(t *testing.T) {
	data := []byte("line1\nline2")
	buf := make([]byte, ReadSize)
	scanner := skim.NewScanner(bytes.NewReader(data), buf, MaxLineSize)
	line, err := scanner.ScanLine()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "line1\n", string(line))

	line, err = scanner.ScanLine()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "line2", string(line))
	line, err = scanner.ScanLine()
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, []byte(nil), line)

	scanner = skim.NewScanner(strings.NewReader("x"), buf, MaxLineSize)
	line, err = scanner.ScanLine()
	require.NoError(t, err)
	require.Equal(t, "x", string(line))
	line, err = scanner.ScanLine()
	require.NoError(t, err)
	require.Equal(t, []byte(nil), line)
}

func TestSkimCarriageReturn(t *testing.T) {
	data := []byte("line1\n\r\nline2\r\n\r\n\rline3\rline3\nline4\r")
	buf := make([]byte, ReadSize)
	scanner := skim.NewScanner(bytes.NewReader(data), buf, MaxLineSize)
	for _, s := range []string{"line1\n", "line2\n", "\rline3\rline3\n", "line4\r"} {
		line, err := scanner.ScanLine()
		require.NoError(t, err)
		assert.Equal(t, s, string(line))
	}
}
