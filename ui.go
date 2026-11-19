package main

import (
	"fmt"
	"strings"
)

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	red     = "\x1b[38;5;203m"
	green   = "\x1b[38;5;120m"
	yellow  = "\x1b[38;5;221m"
	cyan    = "\x1b[38;5;81m"
	magenta = "\x1b[38;5;177m"
	gray    = "\x1b[38;5;245m"
)

func clearScreen() { fmt.Print("\x1b[2J\x1b[H") }

func banner() {
	fmt.Println()
	fmt.Printf("  %s%s◆ FORTNITE MULTI TOOL%s  %sv%s%s\n", bold, magenta, reset, gray, version, reset)
	fmt.Printf("  %s%s%s\n\n", gray, strings.Repeat("─", 48), reset)
}

func okMsg(s string)   { fmt.Printf("  %s✔%s %s\n", green, reset, s) }
func warnMsg(s string) { fmt.Printf("  %s!%s %s\n", yellow, reset, s) }
func failMsg(s string) { fmt.Printf("  %s✖%s %s\n", red, reset, s) }

func ask(prompt string) string {
	fmt.Printf("  %s>%s %s", magenta, reset, prompt)
	return readLine()
}

func pause() {
	fmt.Printf("\n  %sPress Enter to go back...%s", gray, reset)
	readLine()
}

func menuItem(n, name, desc string) {
	fmt.Printf("  %s[%s]%s %s%-22s%s %s%s%s\n", cyan, n, reset, bold, name, reset, gray, desc, reset)
}
