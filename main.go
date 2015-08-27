// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/calmh/psmcli/completion"
	"github.com/calmh/upgrade"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	Version    = "unknown-dev"
	signingKey = []byte(`-----BEGIN EC PUBLIC KEY-----
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBYyrh+aRcAmqSAReHq00XgaJC5Zn3
JC/rlXAM5M2ODmMElLypAtnmUYFJTnmQD1KSwV49GFwFy+iqzNa9AfQ4gQQB+RmV
Q2n12crDe2nU9oI0aPZFkIlqrjA0Ky0jT8rpWhmuRc+Bq8XS4q8mY32RFSaceLXo
N62RXYtkeHL+D41Ct+I=
-----END EC PUBLIC KEY-----`)
)

func main() {
	verbose := flag.Bool("v", false, "Verbose output")
	doUpgrade := flag.Bool("upgrade", false, "Upgrade to latest version")
	flag.Usage = usage
	flag.Parse()
	dst := flag.Arg(0)

	if *doUpgrade {
		rels, err := upgrade.GithubReleases("calmh/psmcli", Version, true, false)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		exp := regexp.MustCompile("-" + runtime.GOOS + "-" + runtime.GOARCH)
		for _, rel := range rels {
			assets := upgrade.MatchingAssets(exp, rel)
			if len(assets) == 1 {
				fmt.Println("Trying", assets[0].Name, "...")
				err = upgrade.ToURL(assets[0].URL, signingKey)
				if err == nil {
					fmt.Println("Upgraded")
					return
				}
				fmt.Println(err)
			}
		}

		os.Exit(1)
	}

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
		fmt.Println(err)
		return
	}
	term.SetSize(h, w)

	user := "default"
	for res.Error.Code == CodeAccessDenied {
		term.SetPrompt("Username: ")
		user, err = term.ReadLine()
		if err != nil {
			fmt.Println(err)
			return
		}
		pass, err := term.ReadPassword("Password: ")
		if err != nil {
			fmt.Println(err)
			return
		}
		res, err = conn.run(command{Method: "system.login", Params: []interface{}{user, pass}})
		if err != nil {
			fmt.Println(err)
			return
		}
		if res.Error.Code != 0 {
			fmt.Println(res.Error.Message)
			fmt.Println()
		} else {
			res, err = conn.run(command{Method: "system.version"})
			if err != nil {
				fmt.Println(err)
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
		fmt.Println(err)
		return
	}
	hostname, ok := res.Result.(string)
	if !ok {
		hostname = "(unknown)"
	}

	fmt.Println("PSM version", version, "at", hostname)

	res, err = conn.run(command{Method: "model.isReadOnly"})
	if err != nil {
		fmt.Println(err)
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

	fmt.Println()

	// Set up tab completion based on announced commands and parameters

	smd, err := conn.smd()
	if err != nil {
		fmt.Println(err)
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
			printHelp(term.Escape)
			continue
		}
		if line == "commands" {
			completer.PrintHelp(term.Escape)
			continue
		}

		cmd, err := parseCommand(line)
		if err != nil {
			fmt.Println(err)
			continue
		}

		cmd.ID = id
		id++

		if *verbose {
			// Print the command locally
			bs, _ := json.Marshal(cmd)
			fmt.Printf("> %s", bs)
		}

		// Execute command on PSM

		res, err := conn.run(cmd)
		if err != nil {
			fmt.Println(err)
			return
		}

		printResponse(res)
	}
}

func usage() {
	fmt.Println("psmcli", Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  psmcli [-v] <host:port>")
	fmt.Println("  psmcli -upgrade")
}

func printResponse(res response) {
	if res.Error.Code != 0 {
		fmt.Printf("Error %d: %s", res.Error.Code, res.Error.Message)
	} else if res.Result != nil {
		switch result := res.Result.(type) {
		case []interface{}:
			for _, res := range result {
				switch res := res.(type) {
				case string, int, json.Number:
					fmt.Println(res)
				default:
					bs, _ := json.MarshalIndent(res, "", "    ")
					fmt.Printf("%s\n\n", bs)
				}
			}

		case map[string]interface{}:
			bs, _ := json.MarshalIndent(result, "", "    ")
			fmt.Printf("%s\n\n", bs)

		default:
			fmt.Println(result)
		}
	}
}

func printHelp(esc *terminal.EscapeCodes) {
	fmt.Println(`Usage:

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
