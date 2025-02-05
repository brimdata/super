// Adapted from https://github.com/logrusorgru/grokky/blob/f28bfe018565ac1e90d93502eae1170006dd1f48/grok.go

package grok

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

var (
	// ErrEmptyName arises when pattern name is an empty string
	ErrEmptyName = errors.New("an empty name")
	// ErrEmptyExpression arises when expression is an empty string
	ErrEmptyExpression = errors.New("an empty expression")
	// ErrAlreadyExist arises when pattern with given name alrady exists
	ErrAlreadyExist = errors.New("the pattern already exist")
	// ErrNotExist arises when pattern with given name doesn't exists
	ErrNotExist = errors.New("pattern doesn't exist")
)

// Host is a patterns collection. Host does not need to be kept around
// after all need patterns are generated
type Host map[string]string

// New returns new empty host
func New() Host { return make(Host) }

// Add a new pattern to the Host. If a pattern name
// already exists the ErrAlreadyExists will be returned.
func (h Host) Add(name, expr string) error {
	if name == "" {
		return ErrEmptyName
	}
	if expr == "" {
		return ErrEmptyExpression
	}
	if _, ok := h[name]; ok {
		return ErrAlreadyExist
	}
	if _, err := h.compileExternal(expr); err != nil {
		return err
	}
	h[name] = expr
	return nil
}

func (h Host) compile(name string) (*Pattern, error) {
	expr, ok := h[name]
	if !ok {
		return nil, ErrNotExist
	}
	return h.compileExternal(expr)
}

var patternRegexp = regexp.MustCompile(`\%\{(\w+)(\:([\w\[\]\.]+)(\:(\w+))?)?}`)

func (h Host) compileExternal(expr string) (*Pattern, error) {
	subs := patternRegexp.FindAllString(expr, -1)
	ts := make(map[string]struct{})
	for _, s := range subs {
		name, sem := split(s)
		if _, ok := h[name]; !ok {
			return nil, fmt.Errorf("the '%s' pattern doesn't exist", name)
		}
		ts[sem] = struct{}{}
	}
	if len(subs) == 0 {
		r, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		p := &Pattern{Regexp: r}
		return p, nil
	}
	spl := patternRegexp.Split(expr, -1)
	msi := make(map[string]int)
	order := 1 // semantic order
	var res string
	for i := 0; i < len(spl)-1; i++ {
		splPart := spl[i]
		order += capCount(splPart)
		sub := subs[i]
		subName, subSem := split(sub)
		p, err := h.compile(subName)
		if err != nil {
			return nil, err
		}
		sub = p.String()
		subNumSubexp := p.NumSubexp()
		subNumSubexp++
		sub = wrap(sub)
		if subSem != "" {
			msi[subSem] = order
		}
		res += splPart + sub
		// add sub semantics to this semantics
		for k, v := range p.s {
			if _, ok := ts[k]; !ok {
				msi[k] = order + v
			}
		}
		order += subNumSubexp
	}
	res += spl[len(spl)-1]
	r, err := regexp.Compile(res)
	if err != nil {
		return nil, err
	}
	p := &Pattern{Regexp: r}
	p.s = msi
	p.order = make(map[int]string)
	for k, v := range msi {
		p.order[v] = k
	}
	return p, nil
}

func split(s string) (name, sem string) {
	ss := patternRegexp.FindStringSubmatch(s)
	if len(ss) >= 2 {
		name = ss[1]
	}
	if len(ss) >= 4 {
		sem = ss[3]
	}
	return
}

func wrap(s string) string { return "(" + s + ")" }

var (
	nonCapLeftRxp  = regexp.MustCompile(`\(\?[imsU\-]*\:`)
	nonCapFlagsRxp = regexp.MustCompile(`\(?[imsU\-]+\)`)
)

func capCount(in string) int {
	leftParens := strings.Count(in, "(")
	nonCapLeft := len(nonCapLeftRxp.FindAllString(in, -1))
	nonCapBoth := len(nonCapFlagsRxp.FindAllString(in, -1))
	escapedLeftParens := strings.Count(in, `\(`)
	return leftParens - nonCapLeft - nonCapBoth - escapedLeftParens
}

// Get pattern by name from the Host.
func (h Host) Get(name string) (*Pattern, error) {
	return h.compile(name)
}

// Compile and get pattern without name (and without adding it to this Host)
func (h Host) Compile(expr string) (*Pattern, error) {
	if expr == "" {
		return nil, ErrEmptyExpression
	}
	return h.compileExternal(expr)
}

type Pattern struct {
	*regexp.Regexp
	s        map[string]int
	order    map[int]string
	keyCache []string
	valCache []string
}

// Parse returns a map of matches on the input. The map can be empty.
func (p *Pattern) Parse(input string) map[string]string {
	ss := p.FindStringSubmatch(input)
	r := make(map[string]string)
	if len(ss) <= 1 {
		return r
	}
	for sem, order := range p.s {
		r[sem] = ss[order]
	}
	return r
}

func (p *Pattern) ParseKeyValues(input string) ([]string, []string, bool) {
	a := p.FindStringSubmatchIndex(input)
	if a == nil {
		return nil, nil, false
	}
	p.valCache = p.valCache[:0]
	p.keyCache = p.keyCache[:0]
	for i := 0; i < len(a)/2; i++ {
		if _, ok := p.order[i]; !ok {
			continue
		}
		if a[i*2] == -1 {
			continue
		}
		p.keyCache = append(p.keyCache, p.order[i])
		p.valCache = append(p.valCache, input[a[i*2]:a[i*2+1]])
	}
	return p.keyCache, p.valCache, true
}

// Names returns all names that this pattern has in order.
func (p *Pattern) Names() (ss []string) {
	ss = make([]string, 0, len(p.s))
	for k := range p.s {
		ss = append(ss, k)
	}
	sort.Slice(ss, func(i, j int) bool {
		return p.s[ss[i]] < p.s[ss[j]]
	})
	return
}

// AddFromReader appends all patterns from the reader to this Host.
func (h Host) AddFromReader(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := h.addFromLine(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

var lineRegexp = regexp.MustCompile(`^(\w+)\s+(.+)$`)

func (h Host) addFromLine(line string) error {
	sub := lineRegexp.FindStringSubmatch(line)
	if len(sub) == 0 { // no match
		return nil
	}
	return h.Add(sub[1], sub[2])
}
