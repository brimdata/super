package srcfiles

import (
	"os"
	"sort"
	"strings"
	"unicode"
)

type List struct {
	Text   string
	Files  []File
	errors ErrorList
}

func (l *List) AddError(msg string, pos, end int) {
	l.errors.Append(l, msg, pos, end)
}

func (l *List) Error() error {
	if len(l.errors) == 0 {
		return nil
	}
	return l.errors
}

func (l *List) FileOf(pos int) File {
	i := sort.Search(len(l.Files), func(i int) bool { return l.Files[i].start > pos }) - 1
	return l.Files[i]
}

// Concat reads in the indicated files and concatenates their content with
// newlines appending the final query text.
func Concat(filenames []string, query string) (*List, error) {
	var b strings.Builder
	var files []File
	var needSep bool
	for _, f := range filenames {
		bb, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		// Skip over empty files.
		if len(bb) > 0 {
			files = append(files, newFile(f, b.Len(), bb))
			// Separate file content with a newline but only when needed.
			if needSep {
				b.WriteByte('\n')
			}
			b.Write(bb)
			needSep = !unicode.IsSpace(rune(bb[len(bb)-1]))
		}
	}
	if query != "" {
		// Empty string is the unnamed query text while the included files all
		// have names.
		files = append(files, newFile("", b.Len(), []byte(query)))
		b.WriteString(query)
	}
	return &List{Text: b.String(), Files: files}, nil
}
