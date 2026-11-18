package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// version is overridden at build time by the release workflow.
var version = "dev"

var in = bufio.NewReader(os.Stdin)

type tool struct {
	name string
	desc string
	run  func()
}

func readLine() string {
	s, err := in.ReadString('\n')
	if err != nil && s == "" {
		os.Exit(0)
	}
	return strings.TrimSpace(s)
}

func main() {
	enableANSI()

	// Add new tools here — they show up in the menu automatically.
	tools := []tool{
		{"Live Ping", "All Fortnite regions, in real time", livePing},
		{"Stretched Resolution", "Edit GameUserSettings.ini (auto backup)", stretchTool},
		{"Restore Backup", "Roll GameUserSettings.ini back", restoreTool},
	}

	for {
		clearScreen()
		banner()
		for i, t := range tools {
			fmt.Printf("  %s[%d]%s %s%-22s%s %s%s%s\n", cyan, i+1, reset, bold, t.name, reset, gray, t.desc, reset)
		}
		fmt.Printf("  %s[0]%s Exit\n\n", cyan, reset)

		c := strings.ToLower(ask(""))
		if c == "0" || c == "q" {
			return
		}
		n, err := strconv.Atoi(c)
		if err != nil || n < 1 || n > len(tools) {
			continue
		}
		tools[n-1].run()
	}
}
