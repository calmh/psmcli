// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"code.google.com/p/go.crypto/ssh/terminal"
)

var (
	Version = "unknown-dev"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	verbose := flag.Bool("v", false, "Verbose output")
	flag.Usage = usage
	flag.Parse()
	dst := flag.Arg(0)

	if dst == "" {
		usage()
		os.Exit(2)
	}

	// Add default port 3994 if it's missing in the dst string

	host, port, err := net.SplitHostPort(dst)
	if err != nil && strings.Contains(err.Error(), "missing port") {
		dst = net.JoinHostPort(dst, "3994")
	} else if err != nil {
		log.Println(err)
		return
	} else if port == "" {
		dst = net.JoinHostPort(host, "3994")
	}

	// Connect to PSM

	conn, err := newConnection(dst)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("psmcli", Version, "connected to", conn.conn.RemoteAddr())
	log.Println("^D to quit")
	log.Println("")

	// Use system.version as dummy call to check if we can proceed without
	// authentication.

	res, err := conn.run(command{Method: "system.version"})
	if err != nil {
		log.Println(err)
		return
	}

	initialPrompt := "$ "
	if res.Error.Code == CodeAccessDenied {
		initialPrompt = "Username: "
	}

	// Set up a terminal

	oldState, err := terminal.MakeRaw(0)
	if err != nil {
		log.Println(err)
		return
	}
	defer terminal.Restore(0, oldState)
	term := terminal.NewTerminal(os.NewFile(0, "terminal"), initialPrompt)

	h, w, err := terminal.GetSize(0)
	if err != nil {
		log.Println(err)
		return
	}
	term.SetSize(h, w)

	user := "default"
	for res.Error.Code == CodeAccessDenied {
		term.SetPrompt("Username: ")
		user, err = term.ReadLine()
		if err != nil {
			log.Println(err)
			return
		}
		pass, err := term.ReadPassword("Password: ")
		if err != nil {
			log.Println(err)
			return
		}
		res, err = conn.run(command{Method: "system.login", Params: []interface{}{user, pass}})
		if err != nil {
			log.Println(err)
			return
		}
		if res.Error.Code != 0 {
			log.Println(res.Error.Message)
			log.Println()
		} else {
			res, err = conn.run(command{Method: "system.version"})
			if err != nil {
				log.Println(err)
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
		log.Println(err)
		return
	}
	hostname, ok := res.Result.(string)
	if !ok {
		hostname = "(unknown)"
	}

	log.Println("PSM version", version, "at", hostname)

	res, err = conn.run(command{Method: "model.isReadOnly"})
	if err != nil {
		log.Println(err)
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

	log.Println()

	// Set up tab completion based on announced commands and parameters

	tabcomp := completer{
		term: term,
	}

	smd, err := conn.smd()
	if err != nil {
		log.Fatal(err)
	}
	tabcomp.importSMD(smd.Result.Services)
	tabcomp.words["help"] = completionWord{}

	term.AutoCompleteCallback = tabcomp.complete

	// Start the REPL

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
			tabcomp.printHelp()
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			log.Println("incomplete command")
			continue
		}

		cmd := parseCommand(fields)

		if *verbose {
			// Print the command locally
			bs, _ := json.Marshal(cmd)
			log.Printf("> %s", bs)
		}

		// Execute command on PSM

		res, err := conn.run(cmd)
		if err != nil {
			log.Println(err)
			return
		}

		printResponse(res)
	}
}

func usage() {
	log.Println("psmcli", Version)
	log.Println()
	log.Println("Usage:")
	log.Println("  psmcli [-v] <host:port>")
}

var nextID int

func parseCommand(fields []string) command {
	// The command is the first two parts joined with a dot.

	cmd := command{
		ID:     nextID,
		Method: fields[0] + "." + fields[1],
	}
	nextID++

	// Look for key=val,key=val sequences among params and make them objects.
	// Not stuff that starts with "(" though, because that might be an LDAP
	// query expression.

	for _, param := range fields[2:] {
		if strings.Contains(param, "=") && !strings.HasPrefix(param, "(") {
			parts := strings.Split(param, ",")
			obj := map[string]string{}
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				obj[kv[0]] = kv[1]
			}
			cmd.Params = append(cmd.Params, obj)
		} else {
			cmd.Params = append(cmd.Params, param)
		}
	}

	return cmd
}

func printResponse(res response) {
	if res.Error.Code != 0 {
		log.Printf("Error %d: %s", res.Error.Code, res.Error.Message)
	} else if res.Result != nil {
		switch result := res.Result.(type) {
		case []interface{}:
			for _, res := range result {
				switch res := res.(type) {
				case string, int, json.Number:
					log.Println(res)
				default:
					bs, _ := json.MarshalIndent(res, "", "    ")
					log.Printf("%s\n\n", bs)
				}
			}

		case map[string]interface{}:
			bs, _ := json.MarshalIndent(result, "", "    ")
			log.Printf("%s\n\n", bs)

		default:
			log.Println(result)
		}
	} else {
		log.Println("OK")
	}
}
