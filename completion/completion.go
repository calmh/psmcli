package completion

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh/terminal"
)

// A Completer provides line completion functionality based on Matchers.
type Completer struct {
	matchers []Matcher
}

// NewCompleter returns a new Completer based on the aggregation of the given
// Matchers.
func NewCompleter(m ...Matcher) Completer {
	return Completer{m}
}

// Complete returns the possible continuation of the given line.
func (c Completer) Complete(line string, pos int) (head string, comps []Word, tail string) {
	line, tail = line[:pos], line[pos:]

	words := strings.Split(line, " ")
	if len(words) == 1 {
		return "", aggrMatch(c.matchers, words[0]), tail
	}

	matchers := c.matchers
	for _, word := range words[:len(words)-1] {
		_, matchers = aggrAccept(matchers, word)
	}

	head = strings.Join(words[:len(words)-1], " ") + " "
	return head, aggrMatch(matchers, words[len(words)-1]), tail
}

func (c Completer) PrintHelp(out io.Writer, esc *terminal.EscapeCodes) {
	for _, m := range c.matchers {
		for _, l := range m.Help(esc) {
			fmt.Fprintln(out, l)
		}
	}
}

// A WordCompleter provides line completion functionality based on Matchers.
type WordCompleter struct {
	Completer
}

// NewWordCompleter returns a new Completer based on the aggregation of the given
// Matchers.
func NewWordCompleter(m ...Matcher) WordCompleter {
	return WordCompleter{
		Completer: NewCompleter(m...),
	}
}

// Complete returns the possible continuation of the given line. This
// satisfies the github.com/peterh/liner.WordCompleter type.
func (c WordCompleter) Complete(line string, pos int) (string, []string, string) {
	head, words, tail := c.Completer.Complete(line, pos)
	return head, wordStrings(words), tail
}

type CallbackCompleter struct {
	Completer
	tabsPressed   int
	searchPos     int
	inPlaceholder bool
}

func NewCallbackCompleter(m ...Matcher) *CallbackCompleter {
	return &CallbackCompleter{
		Completer: NewCompleter(m...),
	}
}

func (c *CallbackCompleter) Complete(line string, pos int, key rune) (string, int, bool) {
	if key != '\t' {
		c.tabsPressed = 0
		if c.inPlaceholder {
			// We are overwriting a tab completion produced placeholder. We
			// should erase the rest of the line (containing the placeholder),
			// add the just pressed key, and that's it.
			line = line[:pos]
			c.inPlaceholder = false
			return line + string(key), pos + 1, true
		}
		return line, pos, false
	}

	if c.tabsPressed == 0 {
		// This is the first tab press after typing characters.
		c.searchPos = pos
	} else if pos <= c.searchPos {
		// User has pressed tab, then moved the cursor to the left and pressed
		// tab again.
		c.tabsPressed = 0
		c.searchPos = pos
	} else {
		// We've already pressed tab at least once. The line reflects our
		// position after tab completion. We need to rewind and search over
		// the last word.
		pos = c.searchPos
	}

	head, words, _ := c.Completer.Complete(line, c.searchPos)
	if len(words) == 0 {
		return line, pos, false
	}
	if len(words) == 1 && !words[0].Placeholder {
		line = head + words[0].Value + " "
		return line, len(line), true
	}

	word := words[c.tabsPressed%len(words)]
	if word.Placeholder {
		c.inPlaceholder = true
		pos = len(head)
	} else {
		pos = len(head + word.Value)
	}
	c.tabsPressed++
	return head + word.Value, pos, true
}

// A Matcher provides word completion functionality.
type Matcher interface {
	Match(word string) []Word
	Accept(word string) (bool, []Matcher)
	AddNext(m Matcher)
	Help(*terminal.EscapeCodes) []string
}

type Word struct {
	Value       string
	Placeholder bool
}

// Literal matches any prefix to String, even the empty string.
type Literal struct {
	Value string
	Next  []Matcher
}

func (l *Literal) Match(word string) []Word {
	if strings.HasPrefix(l.Value, word) {
		return []Word{{l.Value, false}}
	}
	return nil
}

func (l *Literal) Accept(word string) (bool, []Matcher) {
	if word == l.Value {
		return true, l.Next
	}
	return false, nil
}

func (l *Literal) AddNext(m Matcher) {
	l.Next = append(l.Next, m)
}

func (l *Literal) Help(esc *terminal.EscapeCodes) []string {
	lines := []string{l.Value}

	var clines []string
	for _, n := range l.Next {
		clines = append(clines, n.Help(esc)...)
	}

	if len(clines) == 1 {
		lines[0] = lines[0] + " " + clines[0]
	} else {
		lines = append(lines, indent(clines)...)
	}
	return lines
}

// Regexp matches any word that matches Exp.
type Regexp struct {
	Exp         *regexp.Regexp
	Placeholder string
	Optional    bool
	Next        []Matcher
}

func (r *Regexp) Match(word string) []Word {
	p := "<" + r.Placeholder + ">"
	if r.Optional {
		p = "[" + r.Placeholder + "]"
	}
	if word == "" {
		return []Word{{p, true}}
	}
	if r.Exp.MatchString(word) {
		return []Word{{word, false}}
	}
	return nil
}

func (r *Regexp) Accept(word string) (bool, []Matcher) {
	if r.Exp.MatchString(word) {
		return true, r.Next
	}
	return false, nil
}

func (r *Regexp) AddNext(m Matcher) {
	r.Next = append(r.Next, m)
}

func (r *Regexp) Help(esc *terminal.EscapeCodes) []string {
	p := "<" + string(esc.Cyan) + r.Placeholder + string(esc.Reset) + ">"
	if r.Optional {
		p = "[" + string(esc.Green) + r.Placeholder + string(esc.Reset) + "]"
	}
	lines := []string{p}

	var clines []string
	for _, n := range r.Next {
		clines = append(clines, n.Help(esc)...)
	}

	if len(clines) == 1 {
		lines[0] = lines[0] + " " + clines[0]
	} else {
		lines = append(lines, indent(clines)...)
	}
	return lines
}

// Combine aggregates the matches of it's children. If any of the Matchers
// match, Accept will return all the Next. Use this when you have multiple
// choices that all result in the same continuation, e.g.:
//
//           /-- create --\
//  command ---- update ---- <id>
//           \-- delete --/
//
// The "create", "update" and "delete" are the Matchers of the Combine, "<id>"
// is the Next.
type Combine struct {
	Matchers []Matcher
	Next     []Matcher
}

func (s *Combine) Match(word string) []Word {
	return aggrMatch(s.Matchers, word)
}

func (s *Combine) Accept(word string) (bool, []Matcher) {
	ok, _ := aggrAccept(s.Matchers, word)
	if ok {
		return true, s.Next
	}
	return false, nil
}

func (s *Combine) AddNext(m Matcher) {
	s.Next = append(s.Next, m)
}

func (s *Combine) Help(esc *terminal.EscapeCodes) []string {
	var lines []string
	for _, m := range s.Matchers {
		mlines := m.Help(esc)
		for _, n := range s.Next {
			nlines := n.Help(esc)
			if len(nlines) == 1 {
				lines = append(lines, mlines[0]+" "+nlines[0])
			} else {
				lines = append(lines, mlines[0]+" "+nlines[0])
				lines = append(lines, indent(nlines[1:])...)
				lines = append(lines, indent(nlines)...)
			}
		}
	}
	return lines
}

func aggrMatch(ms []Matcher, word string) []Word {
	var res []Word
	for _, m := range ms {
		res = append(res, m.Match(word)...)
	}
	return res
}

func aggrAccept(ms []Matcher, word string) (bool, []Matcher) {
	var ok bool
	var res []Matcher
	for _, m := range ms {
		tok, tres := m.Accept(word)
		ok = ok || tok
		res = append(res, tres...)
	}
	return ok, res
}

func wordStrings(ws []Word) []string {
	if ws == nil {
		return nil
	}
	res := make([]string, len(ws))
	for i := range ws {
		res[i] = ws[i].Value
	}
	return res
}

func indent(lines []string) []string {
	n := make([]string, len(lines))
	for i := range lines {
		n[i] = "    " + lines[i]
	}
	return n
}
