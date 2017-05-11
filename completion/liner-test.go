// +build ignore

package main

import (
	"fmt"
	"regexp"

	"github.com/peterh/liner"
	"kastelo.io/psmcli/completion"
)

func main() {
	line := liner.NewLiner()
	defer line.Close()

	c1 := completion.Literal{
		String: "exit",
	}
	c2 := completion.Literal{
		String: "system",
		Next: []completion.Matcher{
			completion.Literal{
				String: "set",
				Next: []completion.Matcher{
					completion.Regexp{
						Exp:         regexp.MustCompile(`^\d+$`),
						Placeholder: "objectID",
					},
				},
			},
			completion.Literal{
				String: "create",
				Next: []completion.Matcher{
					completion.Regexp{
						Exp:         regexp.MustCompile(`^\d+$`),
						Placeholder: "objectID",
					},
				},
			},
		},
	}
	c3 := completion.Literal{
		String: "add",
		Next: []completion.Matcher{
			completion.Regexp{
				Exp:         regexp.MustCompile(`^\d+$`),
				Placeholder: "int",
				Next: []completion.Matcher{
					completion.Regexp{
						Exp:         regexp.MustCompile(`^\d+$`),
						Placeholder: "int",
					},
				},
			},
		},
	}

	completer := completion.NewWordCompleter(c1, c2, c3)

	line.SetWordCompleter(completer.Complete)
	line.SetTabCompletionStyle(liner.TabPrints)

	for {
		l, err := line.Prompt("> ")
		if err != nil {
			break
		}
		if l == "" {
			continue
		}
		fmt.Println(l)
		line.AppendHistory(l)
	}
}
