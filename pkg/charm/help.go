package charm

import (
	"fmt"
	"strings"

	"github.com/brimdata/super/pkg/terminal"
	"github.com/kr/text"
)

// splitFlags is like strings.Split with a comma and also trims whitespace
func splitFlags(flags string) []string {
	var out []string
	for flag := range strings.SplitSeq(flags, ",") {
		out = append(out, strings.TrimSpace(flag))
	}
	return out
}

// flagMap creates a map that maps a name to a boolean based on the existence
// of that name in the comma-separated string of flags.  Whitespace is removed
// from each name in the flags list.  A map that contains no entries is returned
// for an empty string.
func flagMap(flags string) map[string]bool {
	hidden := make(map[string]bool)
	for _, flag := range splitFlags(flags) {
		hidden[flag] = true
	}
	return hidden
}

func FormatParagraph(body, tab string, lineWidth int) string {
	paragraphs := strings.Split(body, "\n\n")
	var chunks []string
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		paragraph = text.Wrap(paragraph, lineWidth)
		paragraph = strings.ReplaceAll(paragraph, "\n", "\n"+tab)
		chunks = append(chunks, paragraph)
	}
	body = strings.Join(chunks, "\n\n"+tab)
	body = strings.TrimRight(body, " \t\n")
	return tab + body + "\n\n"
}

const tab = "    "

func header(heading string) string {
	boldOn := "\033[1m"
	boldOff := "\033[0m"
	return boldOn + heading + boldOff
}

func helpItem(heading, body string) {
	hdr := header(heading)
	body = hdr + "\n" + tab + body + "\n\n"
	fmt.Print(body)
}

func helpDesc(heading, body string) {
	hdr := header(heading)
	body = tab + body + "\n\n"
	w := terminal.Width()
	lineWidth := w - len(tab) - 5
	if len(body) > lineWidth {
		body = FormatParagraph(body, tab, lineWidth)
	}
	fmt.Print(hdr + "\n" + body)
}

func helpList(heading string, lines []string) {
	hdr := header(heading)
	body := strings.Join(lines, "\n"+tab)
	body = hdr + "\n" + tab + body + "\n\n"
	fmt.Print(body)
}

func getCommands(target *Spec, vflag bool) []string {
	var lines []string
	for _, cmd := range target.children {
		name := cmd.Name
		if cmd.Hidden {
			if vflag {
				name = "[" + name + "]"
			} else {
				continue
			}
		}
		line := name + " - " + cmd.Short
		lines = append(lines, line)
	}
	return lines
}

func buildOptions(path path, parentCmd string, showHidden bool) []string {
	n := len(path)
	if n == 1 {
		return path[0].options(showHidden)
	}
	pathCmd := path[0].spec.Name
	if parentCmd != "" {
		pathCmd = parentCmd + " " + pathCmd
	}
	childOptions := buildOptions(path[1:], parentCmd, showHidden)
	options := path[0].options(showHidden)
	if len(options) == 0 {
		return childOptions
	}
	// add a line separator then add the command path in parens as the header
	// for the next set of flags
	childOptions = append(childOptions, "")
	childOptions = append(childOptions, "["+pathCmd+" flags]")
	for _, option := range options {
		childOptions = append(childOptions, option)
	}
	return childOptions
}

func optionsSection(path []*instance, vflag bool) []string {
	return buildOptions(path, "", vflag)
}

func displayHelp(path path, showHidden bool) {
	spec := path.last().spec
	helpItem("NAME", path.pathname()+" - "+spec.Short)
	helpDesc("USAGE", spec.Usage)
	helpList("OPTIONS", optionsSection(path, showHidden))
	if len(spec.children) > 0 {
		helpList("COMMANDS", getCommands(spec, showHidden))
	}
	helpDesc("DESCRIPTION", spec.Long)
}
