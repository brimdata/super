// Adapted from https://github.com/logrusorgru/grokky/blob/f28bfe018565ac1e90d93502eae1170006dd1f48/pattern_test.go

package grok

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPattern_Parse(t *testing.T) {
	h := New()
	require.NoError(t, h.Add("ONE", `\d`))
	require.NoError(t, h.Add("TWO", `%{ONE:one}-%{ONE:two}`))
	require.NoError(t, h.Add("THREE", `%{ONE:zero}-%{TWO:three}`))
	p, err := h.Get("ONE")
	require.NoError(t, err)
	require.NotNil(t, p.Parse("1"))
	p, err = h.Get("TWO")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"one": "1", "two": "2"}, p.Parse("1-2"))
	p, err = h.Get("THREE")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"one":   "1",
		"two":   "2",
		"zero":  "0",
		"three": "1-2",
	}, p.Parse("0-1-2"))
	require.NoError(t, h.Add("FOUR", `%{TWO:two}`))
	p, err = h.Get("FOUR")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"one": "1", "two": "1-2"}, p.Parse("1-2"))
}

func TestPattern_nestedGroups(t *testing.T) {
	h := New()
	require.NoError(t, h.Add("ONE", `\d`))
	require.NoError(t, h.Add("TWO", `(?:%{ONE:one})-(?:%{ONE:two})?`))
	p, err := h.Get("TWO")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"one": "1", "two": "2"}, p.Parse("1-2"))
	require.Equal(t, map[string]string{"one": "1", "two": ""}, p.Parse("1-"))
}

func TestPattern_Names(t *testing.T) {
	h := New()
	require.NoError(t, h.Add("ONE", `\d`))
	require.NoError(t, h.Add("TWO", `%{ONE:one}-%{ONE:two}`))
	require.NoError(t, h.Add("THREE", `%{ONE:zero}-%{TWO:three}`))
	p, err := h.Get("THREE")
	require.NoError(t, err)
	require.Equal(t, []string{"zero", "three", "one", "two"}, p.Names())
}

func TestPattern_ParseKeyValues(t *testing.T) {
	h := NewBase()
	p, err := h.Compile("%{TIMESTAMP_ISO8601:event_time} %{LOGLEVEL:log_level} %{GREEDYDATA:log_message}")
	require.NoError(t, err)
	_, ss, match := p.ParseKeyValues("2020-09-16T04:20:42.45+01:00 DEBUG This is a sample debug log message")
	require.True(t, match)
	require.Equal(t, []string{"2020-09-16T04:20:42.45+01:00", "DEBUG", "This is a sample debug log message"}, ss)
}

func TestPattern_NamesIgnoreTypeCast(t *testing.T) {
	h := New()
	require.NoError(t, h.Add("ONE", `\d`))
	p, err := h.Compile("%{ONE:one:int}")
	require.NoError(t, err)
	require.Equal(t, []string{"one"}, p.Names())
}

func TestPattern_NamesNested(t *testing.T) {
	h := New()
	require.NoError(t, h.Add("ONE", `\d`))
	p, err := h.Compile("%{ONE:num.one}-%{ONE:[num][two]}")
	require.NoError(t, err)
	require.Equal(t, []string{"num.one", "[num][two]"}, p.Names())
}

func TestPattern_OptionalValues(t *testing.T) {
	h := NewBase()
	p, err := h.Compile("(%{INT:a}|%{INT:b})")
	require.NoError(t, err)
	keys, values, match := p.ParseKeyValues("1")
	require.True(t, match)
	require.Equal(t, []string{"a"}, keys)
	require.Equal(t, []string{"1"}, values)
}
