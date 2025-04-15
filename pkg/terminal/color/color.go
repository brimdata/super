package color

import (
	"fmt"
)

var Enabled = false

type Code int

var (
	Reset      Code = -1
	Bold       Code = -2
	Red        Code = 1
	Green      Code = 2
	GrayYellow Code = 3
	Blue       Code = 4
	Turqoise   Code = 31
	Purple     Code = 105
	Orange     Code = 208
	Pink       Code = 200
)

func (code Code) String() string {
	if Enabled {
		if code == Reset {
			return "\u001b[0m"
		}
		if code == Bold {
			return "\u001b[1m"
		}
		return fmt.Sprintf("\u001b[38;5;%dm", code)
	}
	return ""
}

func (code Code) Colorize(s string) string {
	if !Enabled {
		return s
	}
	return code.String() + s + Reset.String()
}

func Embolden(s string) string {
	if !Enabled {
		return s
	}
	return Bold.Colorize(s)
}

func Gray(level int) Code {
	if level < 0 {
		level = 0
	} else if level > 255 {
		level = 24
	} else {
		level = (level * 24) / 255
	}
	return Code(255 - 24 + level)
}

func Palette() string {
	var out string
	for i := range 16 {
		for j := range 16 {
			code := i*16 + j
			out += Code(code).String()
			out += fmt.Sprintf(" %d", code)
		}
	}
	out += Reset.String()
	return out
}
