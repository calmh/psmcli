// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"regexp"
	"sort"
	"strings"

	"kastelo.io/psmcli/completion"
)

func importSMD(services map[string]smdService) []completion.Matcher {
	var matchers []completion.Matcher
	var svcs []string
	for name := range services {
		svcs = append(svcs, name)
	}
	sort.Strings(svcs)

	roots := make(map[string]completion.Matcher)

	for _, svc := range svcs {
		parts := strings.SplitN(svc, ".", 2)
		if len(parts) != 2 {
			continue
		}

		cmd := &completion.Literal{
			Value: parts[1],
		}
		if root, ok := roots[parts[0]]; ok {
			root.AddNext(cmd)
		} else {
			root = &completion.Literal{Value: parts[0]}
			roots[parts[0]] = root
			matchers = append(matchers, root)
			root.AddNext(cmd)
		}

		var cur completion.Matcher = cmd
		for _, param := range services[svc].Parameters {
			if param.Name == "type" && !param.Optional && param.Type == "string" {
				s := &completion.Combine{
					Matchers: []completion.Matcher{
						&completion.Literal{Value: "session"},
						&completion.Literal{Value: "subscriber"},
						&completion.Literal{Value: "group"},
					},
				}
				cur.AddNext(s)
				cur = s
			} else {
				var m completion.Matcher
				switch param.Type {
				case "integer":
					m = &completion.Regexp{
						Exp:         regexp.MustCompile(`^\d+$`),
						Placeholder: param.Name + " (int)",
						Optional:    param.Optional,
					}
				default:
					m = &completion.Regexp{
						Exp:         regexp.MustCompile(`.`),
						Placeholder: param.Name + " (" + param.Type + ")",
						Optional:    param.Optional,
					}
				}
				cur.AddNext(m)
				cur = m
			}
		}
	}

	return matchers
}
