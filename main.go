// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"
	"kastelo.io/psmcli/completion"
)

var (
	Version = "unknown-dev"
)

func main() {
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Usage = usage
	flag.Parse()
	dst := flag.Arg(0)

	if dst == "" {
		usage()
		os.Exit(2)
	}

	fmt.Println("psmcli", Version)
	fmt.Println("^D to quit")

	// Add default port 3994 if it's missing in the dst string

	host, port, err := net.SplitHostPort(dst)
	if err != nil && strings.Contains(err.Error(), "missing port") {
		dst = net.JoinHostPort(dst, "3994")
	} else if err != nil {
		fmt.Println(err)
		return
	} else if port == "" {
		dst = net.JoinHostPort(host, "3994")
	}

	// Connect to PSM

	conn, err := newConnection(dst)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Connected to", conn.conn.RemoteAddr())
	fmt.Println("")

	// Use system.version as dummy call to check if we can proceed without
	// authentication.

	res, err := conn.run(command{Method: "system.version"})
	if err != nil {
		fmt.Println(err)
		return
	}

	initialPrompt := "$ "
	if res.Error.Code == CodeAccessDenied {
		initialPrompt = "Username: "
	}

	// Set up a terminal

	oldState, err := terminal.MakeRaw(0)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		terminal.Restore(0, oldState)
		fmt.Println("")
	}()

	term := terminal.NewTerminal(os.NewFile(0, "terminal"), initialPrompt)

	h, w, err := terminal.GetSize(0)
	if err != nil {
		fmt.Fprintln(term, err)
		return
	}
	term.SetSize(h, w)

	user := "default"
	for res.Error.Code == CodeAccessDenied {
		term.SetPrompt("Username: ")
		user, err = term.ReadLine()
		if err != nil {
			fmt.Fprintln(term, err)
			return
		}
		pass, err := term.ReadPassword("Password: ")
		if err != nil {
			fmt.Fprintln(term, err)
			return
		}
		res, err = conn.run(command{Method: "system.login", Params: []interface{}{user, pass}})
		if err != nil {
			fmt.Fprintln(term, err)
			return
		}
		if res.Error.Code != 0 {
			fmt.Fprintln(term, res.Error.Message)
			fmt.Fprintln(term)
		} else {
			res, err = conn.run(command{Method: "system.version"})
			if err != nil {
				fmt.Fprintln(term, err)
				return
			}
		}
	}

	// Print version and hostname as identification

	version, ok := res.Result.(string)
	if !ok {
		version = "(unknown)"
	}

	res, err = conn.run(command{Method: "system.hostname"})
	if err != nil {
		fmt.Fprintln(term, err)
		return
	}
	hostname, ok := res.Result.(string)
	if !ok {
		hostname = "(unknown)"
	}

	fmt.Fprintln(term, "PSM version", version, "at", hostname)

	res, err = conn.run(command{Method: "model.isReadOnly"})
	if err != nil {
		fmt.Fprintln(term, err)
		return
	}

	// Set up the prompt as user@host, root-style if the model is read/write
	// otherwise user-style.

	var roRw = " # "
	if ro, ok := res.Result.(bool); ok && ro {
		roRw = " $ "
	}

	hostnameParts := strings.SplitN(hostname, ".", 2)
	hostname = hostnameParts[0]
	term.SetPrompt(user + "@" + hostname + roRw)

	fmt.Fprintln(term)

	// Set up tab completion based on announced commands and parameters

	smd, err := conn.smd()
	if err != nil {
		fmt.Fprintln(term, err)
		os.Exit(1)
	}

	matchers := importSMD(smd.Result.Services)
	completer := completion.NewCallbackCompleter(matchers...)
	term.AutoCompleteCallback = completer.Complete

	// Start the REPL

	id := 0
	for {
		line, err := term.ReadLine()
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "help" || line == "?" {
			printHelp(term, term.Escape)
			continue
		}
		if line == "commands" {
			completer.PrintHelp(term, term.Escape)
			continue
		}

		cmd, err := parseCommand(line)
		if err != nil {
			fmt.Fprintln(term, err)
			continue
		}

		cmd.ID = id
		id++

		if *verbose {
			// Print the command locally
			bs, _ := json.Marshal(cmd)
			fmt.Fprintf(term, "> %s\n", bs)
		}

		// Execute command on PSM

		res, err := conn.run(cmd)
		if err != nil {
			fmt.Fprintln(term, err)
			return
		}

		printResponse(term, res)
	}
}

func usage() {
	fmt.Println("psmcli", Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  psmcli [-v] <host:port>")
}

func printResponse(out io.Writer, res response) {
	if res.Error.Code != 0 {
		fmt.Fprintf(out, "Error %d: %s", res.Error.Code, res.Error.Message)
	} else if res.Result != nil {
		switch result := res.Result.(type) {
		case []interface{}:
			for _, res := range result {
				switch res := res.(type) {
				case string, int, json.Number:
					fmt.Fprintln(out, res)
				default:
					bs, _ := json.MarshalIndent(res, "", "    ")
					fmt.Fprintf(out, "%s\n\n", bs)
				}
			}

		case map[string]interface{}:
			bs, _ := json.MarshalIndent(result, "", "    ")
			fmt.Fprintf(out, "%s\n\n", bs)

		default:
			fmt.Fprintln(out, result)
		}
	}
}

func printHelp(out io.Writer, esc *terminal.EscapeCodes) {
	fmt.Fprintln(out, `Usage:

help, ?:
	Print this help

commands:
	Print available PSM commands. Commands have tab completion available.

Examples:

Simple command without parameter:
	$ system hostname

Command with parameters:
	$ object deleteByAid subscriber 1234

Command with object parameters:
	$ object updateByAid subscriber 1234 attr=value
	$ object updateByAid subscriber 1234 attr1=value1,attr2=value2

	(No spaces in the object parameter!)

Command with arbitrary JSON object parameter:
	$ object updateByAid subscriber 1234
		{"attr1": "value1 with space", "attr2": "value2"}

	(Line break for display purposes only)
`)
}
