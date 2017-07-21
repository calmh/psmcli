package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

type wordOrJSONScanner struct {
	inJSON bool
}

func (s *wordOrJSONScanner) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	start := 0
	if !s.inJSON {
		// Skip leading spaces.
		for width := 0; start < len(data); start += width {
			var r rune
			r, width = utf8.DecodeRune(data[start:])
			if !isSpace(r) {
				break
			}
		}
	}

	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if r == '{' && i == 0 && !s.inJSON {
			// The first rune of the word is a {, so it's the start of a JSON
			// object.
			s.inJSON = true
			continue
		}
		if !s.inJSON && isSpace(r) {
			return i + width, data[start:i], nil
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}

func parseCommand(line string) (command, error) {
	var fields []string
	var splitter wordOrJSONScanner

	s := bufio.NewScanner(bytes.NewReader([]byte(line)))
	s.Split(splitter.split)

	for s.Scan() {
		fields = append(fields, s.Text())
	}
	if err := s.Err(); err != nil {
		return command{}, err
	}

	if len(fields) < 2 {
		return command{}, errors.New("incomplete command")
	}

	// The command is the first two parts joined with a dot.

	cmd := command{
		Method: fields[0] + "." + fields[1],
	}

	// Look for key=val,key=val sequences among params and make them objects.
	// Not stuff that starts with "(" though, because that might be an LDAP
	// query expression.

	for _, param := range fields[2:] {
		if strings.HasPrefix(param, "{") {
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(param), &obj)
			if err != nil {
				return command{}, err
			}
			cmd.Params = append(cmd.Params, obj)
		} else if strings.Contains(param, "=") && !strings.HasPrefix(param, "(") {
			parts := strings.Split(param, ",")
			obj := map[string]string{}
			for _, part := range parts {
				if !strings.Contains(part, "=") {
					obj[part] = ""
					continue
				}
				kv := strings.SplitN(part, "=", 2)
				obj[kv[0]] = kv[1]
			}
			cmd.Params = append(cmd.Params, obj)
		} else {
			cmd.Params = append(cmd.Params, param)
		}
	}

	return cmd, nil
}

// isSpace reports whether the character is a Unicode white space character.
// We avoid dependency on the unicode package, but check validity of the implementation
// in the tests.
func isSpace(r rune) bool {
	if r <= '\u00FF' {
		// Obvious ASCII ones: \t through \r plus space. Plus two Latin-1 oddballs.
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return true
		case '\u0085', '\u00A0':
			return true
		}
		return false
	}
	// High-valued ones.
	if '\u2000' <= r && r <= '\u200a' {
		return true
	}
	switch r {
	case '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	}
	return false
}
