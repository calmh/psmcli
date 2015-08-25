package completion

import (
	"reflect"
	"regexp"
	"testing"
)

var _ Matcher = &Literal{}
var _ Matcher = &Regexp{}
var _ Matcher = &Combine{}

func TestLiteral(t *testing.T) {
	testcases := []struct {
		literal string
		input   string
		matches []Word
	}{
		{"foo", "foo", []Word{{"foo", false}}},
		{"foo", "f", []Word{{"foo", false}}},
		{"foo", "", []Word{{"foo", false}}},
		{"foo", "g", nil},
	}

	for _, tc := range testcases {
		l := Literal{Value: tc.literal}
		m := l.Match(tc.input)
		if !reflect.DeepEqual(m, tc.matches) {
			t.Errorf("Incorrect match: literal %q, input %q, matches: %+v != %+v", tc.literal, tc.input, m, tc.matches)
		}
	}
}

func TestWordCompleter(t *testing.T) {
	c1 := &Literal{
		Value: "foo",
		Next: []Matcher{
			&Literal{Value: "bar"},
			&Literal{Value: "baz"},
		},
	}
	c2 := &Literal{
		Value: "goo",
		Next: []Matcher{
			&Literal{Value: "quux"},
			&Literal{Value: "flaa"},
			&Regexp{
				Exp:         regexp.MustCompile(`^\d+$`),
				Placeholder: "integer",
				Next: []Matcher{
					&Literal{Value: "end"},
				},
			},
		},
	}

	type input struct {
		line string
		pos  int
	}
	type res struct {
		head  string
		comps []string
		tail  string
	}
	testcases := []struct {
		input
		res
	}{
		{input{"", 0},
			res{"", []string{"foo", "goo"}, ""}},
		{input{"f", 1},
			res{"", []string{"foo"}, ""}},
		{input{"foo", 3},
			res{"", []string{"foo"}, ""}},
		{input{"foo ", 4},
			res{"foo ", []string{"bar", "baz"}, ""}},
		{input{"foo ba", 6},
			res{"foo ", []string{"bar", "baz"}, ""}},
		{input{"foo g", 5},
			res{"foo ", nil, ""}},
		{input{"goo ", 4},
			res{"goo ", []string{"quux", "flaa", "<integer>"}, ""}},
		{input{"goo q", 5},
			res{"goo ", []string{"quux"}, ""}},
		{input{"goo 1", 5},
			res{"goo ", []string{"1"}, ""}},
		{input{"goo 142", 7},
			res{"goo ", []string{"142"}, ""}},
		{input{"goo 142 ", 8},
			res{"goo 142 ", []string{"end"}, ""}},
		{input{"goo 142a", 8},
			res{"goo ", nil, ""}},
	}

	c := NewWordCompleter(c1, c2)
	for _, tc := range testcases {
		t.Log(tc.input)
		head, comps, tail := c.Complete(tc.input.line, tc.input.pos)
		if head != tc.res.head {
			t.Errorf("Incorrect head: input %v, %q != %q", tc.input, head, tc.res.head)
		}
		if !reflect.DeepEqual(comps, tc.res.comps) {
			t.Errorf("Incorrect completion: input %v, completions %+v != %+v", tc.input, comps, tc.res.comps)
		}
		if tail != tc.res.tail {
			t.Errorf("Incorrect tail: input %v, %q != %q", tc.input, tail, tc.res.tail)
		}
	}
}

func TestCallbackCompleter(t *testing.T) {
	c1 := &Literal{
		Value: "foo",
		Next: []Matcher{
			&Literal{Value: "bar"},
			&Literal{Value: "baz"},
		},
	}
	c2 := &Literal{
		Value: "goo",
		Next: []Matcher{
			&Literal{Value: "quux"},
			&Literal{Value: "flaa"},
			&Regexp{
				Exp:         regexp.MustCompile(`^\d+$`),
				Placeholder: "integer",
				Next: []Matcher{
					&Literal{Value: "end"},
				},
			},
		},
	}

	type input struct {
		line string
		pos  int
		key  rune
	}
	type res struct {
		line    string
		pos     int
		handled bool
	}
	testcases := []struct {
		input
		res
	}{
		{input{"fo", 2, 'o'}, res{"fo", 2, false}},
		{input{"fo", 2, '\t'}, res{"foo ", 4, true}},
		// ---
		{input{"foo", 3, ' '}, res{"foo", 3, false}},
		{input{"foo ", 4, '\t'}, res{"foo bar", 7, true}},
		{input{"foo bar", 7, '\t'}, res{"foo baz", 7, true}},
		{input{"foo baz", 7, '\t'}, res{"foo bar", 7, true}},
		{input{"foo ", 4, '\t'}, res{"foo bar", 7, true}},
		// ---
		{input{"goo", 3, ' '}, res{"goo", 3, false}},
		{input{"goo ", 4, '\t'}, res{"goo quux", 8, true}},
		{input{"goo quux", 8, '\t'}, res{"goo flaa", 8, true}},
		{input{"goo flaa", 8, '\t'}, res{"goo <integer>", 4, true}},
		{input{"goo ", 4, '1'}, res{"goo 1", 5, true}},
	}

	c := NewCallbackCompleter(c1, c2)
	for _, tc := range testcases {
		t.Log(tc.input)
		line, pos, handled := c.Complete(tc.input.line, tc.input.pos, tc.input.key)
		if line != tc.res.line {
			t.Errorf("Incorrect line: input %v, %q != %q", tc.input, line, tc.res.line)
		}
		if pos != tc.res.pos {
			t.Errorf("Incorrect pos: input %v, %d != %d", tc.input, pos, tc.res.pos)
		}
		if handled != tc.res.handled {
			t.Errorf("Incorrect handled: input %v, %v != %v", tc.input, handled, tc.res.handled)
		}
	}
}
