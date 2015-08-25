// +build ignore

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/calmh/psmcli/completion"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	oldState, err := terminal.MakeRaw(0)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		terminal.Restore(0, oldState)
		fmt.Println("")
	}()

	term := terminal.NewTerminal(os.NewFile(0, "terminal"), "> ")

	h, w, err := terminal.GetSize(0)
	if err != nil {
		fmt.Println(err)
		return
	}
	term.SetSize(h, w)

	c1 := completion.Literal{
		String: "foo",
		Next: []completion.Matcher{
			completion.Literal{String: "bar"},
			completion.Literal{String: "baz"},
		},
	}
	c2 := completion.Literal{
		String: "goo",
		Next: []completion.Matcher{
			completion.Literal{String: "quux"},
			completion.Literal{String: "flaa"},
			completion.Regexp{
				Exp:         regexp.MustCompile(`^\d+$`),
				Placeholder: "integer",
				Next: []completion.Matcher{
					completion.Literal{String: "end"},
				},
			},
		},
	}

	completer := completion.NewCallbackCompleter(c1, c2)

	term.AutoCompleteCallback = completer.Complete

	for {
		line, err := term.ReadLine()
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fmt.Println(line)
	}
}
