package zfmt

import (
	"fmt"
	"strings"
)

type formatter struct {
	strings.Builder
	indent  int
	tab     int
	needTab bool
	needRet bool
}

func (f *formatter) flush() {
	if f.needRet {
		f.WriteByte('\n')
		f.needRet = false
	}
}

func (f *formatter) writeTab() {
	f.flush()
	for k := 0; k < f.indent; k++ {
		f.WriteByte(' ')
	}
	f.needTab = false
}

func (f *formatter) write(args ...any) {
	f.flush()
	if f.needTab {
		f.writeTab()
	}
	var s string
	if len(args) == 1 {
		s = args[0].(string)
	} else if len(args) > 1 {
		format := args[0].(string)
		s = fmt.Sprintf(format, args[1:]...)
	}
	f.WriteString(s)
}

func (f *formatter) open(args ...any) {
	if len(args) > 0 {
		f.write(args...)
	}
	f.indent += f.tab
}

func (f *formatter) close() {
	f.indent -= f.tab
}

func (f *formatter) ret() {
	f.needTab = true
	f.needRet = true
}

func (f *formatter) space() {
	f.write(" ")
}
