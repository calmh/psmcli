// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"code.google.com/p/go.crypto/ssh/terminal"
)

type completionMap map[string]completionWord

type completionWord struct {
	placeholder bool // should be replaced by actual text by the user
	optional    bool // triggers "optional"-style formatting
	next        completionMap
}

type completer struct {
	term          *terminal.Terminal
	inProgress    bool
	tabsPressed   int
	searchPos     int
	inPlaceholder bool
	words         completionMap
}

func (c *completer) importSMD(services map[string]smdService) {
	c.words = make(completionMap)
	for name, service := range services {
		parts := strings.Split(name, ".")
		words := c.words
		for _, part := range parts {
			word, ok := words[part]
			if !ok {
				word.next = make(completionMap)
				words[part] = word
			}
			words = word.next
		}
		for _, param := range service.Parameters {
			if param.Name == "type" && !param.Optional && param.Type == "string" {
				// Handle "type" parameter specifically, as it can always expand to either "session", "subscriber" or "group".
				next := make(completionMap)
				for _, s := range []string{"session", "subscriber", "group"} {
					word := completionWord{
						placeholder: false,
						optional:    false,
						next:        next,
					}
					words[s] = word
				}
				words = next
			} else {
				// Add the parameter as a placeholder
				word := completionWord{
					placeholder: true,
					optional:    param.Optional,
					next:        make(completionMap),
				}
				words[param.Name] = word
				words = word.next
			}
		}
	}
}

type completionMatch struct {
	line        string
	pos         int
	placeholder bool
}

type completionMatchSlice []completionMatch

func (s completionMatchSlice) Len() int {
	return len(s)
}

func (s completionMatchSlice) Less(a, b int) bool {
	return s[a].line < s[b].line
}

func (s completionMatchSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

func (c *completer) complete(line string, pos int, key rune) (string, int, bool) {
	if key != '\t' {
		c.inProgress = false
		if c.inPlaceholder {
			line = line[:pos]
			c.inPlaceholder = false
			return fmt.Sprintf("%s%c", line, key), pos + 1, true
		}
		return line, pos, false
	}

	if !c.inProgress || pos < c.searchPos {
		c.tabsPressed = 0
		c.inProgress = true
		c.searchPos = pos
	}

	search := line[:c.searchPos]
	allParts := strings.Fields(search)
	if len(allParts) == 0 {
		return line, pos, false
	}
	if line[c.searchPos-1] == ' ' {
		allParts = append(allParts, "")
	}

	firstParts := allParts[:len(allParts)-1]
	lastPart := allParts[len(allParts)-1]
	words := c.words
	for _, part := range firstParts {
		word, found := words[part]
		if !found {
			return line, pos, false
		}
		words = word.next
	}

	var matches completionMatchSlice
	for word, comp := range words {
		if strings.HasPrefix(word, lastPart) {
			if comp.placeholder {
				// For placeholders, put the cursor at the start of the placeholder
				newLine := strings.Join(firstParts, " ")
				fullWord := "<" + word + ">"
				if comp.optional {
					fullWord = "[" + word + "]"
				}
				matches = append(matches, completionMatch{
					line:        newLine + " " + fullWord,
					pos:         len(newLine + " "),
					placeholder: true,
				})
			} else {
				// For regular matches, put the cursor at the end of the word
				newLine := strings.Join(append(firstParts, word), " ")
				matches = append(matches, completionMatch{
					line: newLine,
					pos:  len(newLine),
				})
			}
		}
	}

	if len(matches) == 0 {
		return line, pos, false
	}

	if len(matches) == 1 && !matches[0].placeholder {
		// Treat the only match specially, by appending a space and moving to
		// the next word.
		match := matches[0]
		c.inProgress = false
		c.inPlaceholder = false
		return match.line + " ", match.pos + 1, true
	}

	sort.Sort(matches)
	match := matches[c.tabsPressed%len(matches)]
	c.inPlaceholder = match.placeholder
	c.tabsPressed++
	return match.line, match.pos, true
}

func (c *completer) printHelp() {
	var commands []string
	for word, comp := range c.words {
		for subword := range comp.next {
			commands = append(commands, word+" "+subword)
		}
	}
	sort.Strings(commands)
	for _, cmd := range commands {
		log.Println(cmd)
	}
}
