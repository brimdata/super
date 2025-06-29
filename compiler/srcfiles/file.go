package srcfiles

import (
	"sort"
)

// File holds source file offsets.
type File struct {
	Name  string
	lines []int
	size  int
	start int
}

func newFile(name string, start int, src []byte) File {
	var lines []int
	line := 0
	for offset, b := range src {
		if line >= 0 {
			lines = append(lines, line)
		}
		line = -1
		if b == '\n' {
			line = offset + 1
		}
	}
	return File{
		Name:  name,
		lines: lines,
		size:  len(src),
		start: start,
	}
}

func (f File) Position(pos int) Position {
	if pos < 0 {
		return Position{-1, -1, -1, -1}
	}
	offset := pos - f.start
	i := searchLine(f.lines, offset)
	if i < 0 {
		return Position{-1, -1, -1, -1}
	}
	return Position{
		Pos:    pos,
		Offset: offset,
		Line:   i + 1,
		Column: offset - f.lines[i] + 1,
	}
}

func (f File) LineOfPos(src string, pos int) string {
	i := searchLine(f.lines, pos-f.start)
	start := f.lines[i]
	end := f.size
	if i+1 < len(f.lines) {
		end = f.lines[i+1]
	}
	b := src[f.start+start : f.start+end]
	if b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func searchLine(lines []int, offset int) int {
	return sort.Search(len(lines), func(i int) bool { return lines[i] > offset }) - 1

}

type Position struct {
	Pos    int `json:"pos"`    // Offset relative to entire source text in List.Text.
	Offset int `json:"offset"` // Offset relative to local source text in this File.
	Line   int `json:"line"`   // 1-based line number.
	Column int `json:"column"` // 1-based column number.
}

func (p Position) IsValid() bool { return p.Pos >= 0 }
