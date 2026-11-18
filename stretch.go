package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const section = "[/Script/FortniteGame.FortGameUserSettings]"

type res struct {
	w, h  int
	label string
}

var presets = []res{
	{1920, 1080, "Native 16:9"},
	{1680, 1050, "16:10 stretch"},
	{1440, 1080, "4:3 stretch"},
	{1280, 1024, "5:4 stretch"},
	{1280, 960, "4:3 stretch (low)"},
	{1080, 1080, "1:1 stretch"},
}

func settingsPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "FortniteGame", "Saved", "Config", "WindowsClient", "GameUserSettings.ini")
}

func backupDir() string {
	return filepath.Join(filepath.Dir(settingsPath()), "multitool-backups")
}

func fortniteRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq FortniteClient-Win64-Shipping.exe", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "fortniteclient")
}

func splitLines(s string) ([]string, bool) {
	crlf := strings.Contains(s, "\r\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n"), crlf
}

func joinLines(lines []string, crlf bool) string {
	sep := "\n"
	if crlf {
		sep = "\r\n"
	}
	return strings.Join(lines, sep)
}

// sectionRange returns the index of the section header and the end (exclusive) of its body.
func sectionRange(lines []string) (int, int) {
	start := -1
	for i, l := range lines {
		if strings.EqualFold(strings.TrimSpace(l), section) {
			start = i
			break
		}
	}
	if start == -1 {
		return -1, -1
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}
	return start, end
}

func getVal(lines []string, key string) string {
	start, end := sectionRange(lines)
	if start == -1 {
		return "?"
	}
	for i := start + 1; i < end; i++ {
		kv := strings.SplitN(lines[i], "=", 2)
		if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), key) {
			return strings.TrimSpace(kv[1])
		}
	}
	return "?"
}

func setKeys(lines []string, kv [][2]string) []string {
	start, _ := sectionRange(lines)
	if start == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, section)
	}
	for _, p := range kv {
		start, end := sectionRange(lines)
		found := false
		for i := start + 1; i < end; i++ {
			parts := strings.SplitN(lines[i], "=", 2)
			if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), p[0]) {
				lines[i] = p[0] + "=" + p[1]
				found = true
				break
			}
		}
		if !found {
			ins := start + 1
			tail := append([]string{p[0] + "=" + p[1]}, lines[ins:]...)
			lines = append(lines[:ins], tail...)
		}
	}
	return lines
}

func parseRes(s string) (int, int) {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil {
		return 0, 0
	}
	return w, h
}

func backupFile(data []byte) (string, error) {
	dir := backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "GameUserSettings_"+time.Now().Format("20060102-150405")+".ini")
	return dst, os.WriteFile(dst, data, 0o644)
}

func stretchTool() {
	clearScreen()
	banner()
	path := settingsPath()
	if path == "" {
		failMsg("LOCALAPPDATA is not set (this tool is Windows only).")
		pause()
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		failMsg("Could not read GameUserSettings.ini:")
		fmt.Printf("    %s%s%s\n", gray, path, reset)
		warnMsg("Launch Fortnite once so the file gets created.")
		pause()
		return
	}
	lines, crlf := splitLines(string(data))

	fmt.Printf("  %sFile:%s %s\n", gray, reset, path)
	fmt.Printf("  %sCurrent:%s %sx%s  (FullscreenMode=%s)\n\n", gray, reset,
		getVal(lines, "ResolutionSizeX"), getVal(lines, "ResolutionSizeY"), getVal(lines, "FullscreenMode"))
	for i, p := range presets {
		fmt.Printf("  %s[%d]%s %dx%d  %s%s%s\n", cyan, i+1, reset, p.w, p.h, gray, p.label, reset)
	}
	fmt.Printf("  %s[c]%s Custom\n  %s[0]%s Back\n\n", cyan, reset, cyan, reset)

	choice := strings.ToLower(ask("Select: "))
	var w, h int
	switch {
	case choice == "0" || choice == "":
		return
	case choice == "c":
		w, h = parseRes(ask("Resolution (e.g. 1728x1080): "))
	default:
		n, convErr := strconv.Atoi(choice)
		if convErr != nil || n < 1 || n > len(presets) {
			return
		}
		w, h = presets[n-1].w, presets[n-1].h
	}
	if w < 640 || h < 480 || w > 7680 || h > 4320 {
		failMsg("Invalid resolution.")
		pause()
		return
	}

	if fortniteRunning() {
		warnMsg("Fortnite is running. Close it first, or it will overwrite the change.")
		if !strings.EqualFold(ask("Continue anyway? (y/N): "), "y") {
			return
		}
	}

	bak, err := backupFile(data)
	if err != nil {
		failMsg("Backup failed: " + err.Error())
		pause()
		return
	}
	okMsg("Backup saved: " + filepath.Base(bak))

	sw, sh := strconv.Itoa(w), strconv.Itoa(h)
	lines = setKeys(lines, [][2]string{
		{"ResolutionSizeX", sw},
		{"ResolutionSizeY", sh},
		{"LastUserConfirmedResolutionSizeX", sw},
		{"LastUserConfirmedResolutionSizeY", sh},
		{"DesiredScreenWidth", sw},
		{"DesiredScreenHeight", sh},
		{"LastUserConfirmedDesiredScreenWidth", sw},
		{"LastUserConfirmedDesiredScreenHeight", sh},
		{"FullscreenMode", "0"},
		{"LastConfirmedFullscreenMode", "0"},
		{"PreferredFullscreenMode", "0"},
	})

	_ = os.Chmod(path, 0o666)
	if err := os.WriteFile(path, []byte(joinLines(lines, crlf)), 0o666); err != nil {
		failMsg("Could not write file: " + err.Error())
		pause()
		return
	}
	okMsg(fmt.Sprintf("Resolution set to %dx%d (fullscreen mode).", w, h))

	if strings.EqualFold(ask("Make the file read-only so Fortnite can't revert it? (y/N): "), "y") {
		if err := os.Chmod(path, 0o444); err == nil {
			okMsg("File is read-only now. Run this tool again to unlock it.")
		}
	}
	pause()
}

func restoreTool() {
	clearScreen()
	banner()
	path := settingsPath()
	if path == "" {
		failMsg("LOCALAPPDATA is not set (this tool is Windows only).")
		pause()
		return
	}
	entries, err := os.ReadDir(backupDir())
	var names []string
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".ini") {
				names = append(names, e.Name())
			}
		}
	}
	if len(names) == 0 {
		warnMsg("No backups yet.")
		pause()
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) > 10 {
		names = names[:10]
	}
	for i, n := range names {
		fmt.Printf("  %s[%d]%s %s\n", cyan, i+1, reset, n)
	}
	fmt.Println()

	n, convErr := strconv.Atoi(ask("Restore which backup? (0 = back): "))
	if convErr != nil || n < 1 || n > len(names) {
		return
	}
	data, err := os.ReadFile(filepath.Join(backupDir(), names[n-1]))
	if err != nil {
		failMsg("Could not read backup: " + err.Error())
		pause()
		return
	}
	if cur, readErr := os.ReadFile(path); readErr == nil {
		_, _ = backupFile(cur)
	}
	_ = os.Chmod(path, 0o666)
	if err := os.WriteFile(path, data, 0o666); err != nil {
		failMsg("Could not write file: " + err.Error())
		pause()
		return
	}
	okMsg("Restored " + names[n-1])
	pause()
}
