package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// run executes a program and returns combined stdout+stderr.
func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// ps runs a PowerShell snippet (UTF-8 output). Use single quotes inside scripts.
func ps(script string) (string, error) {
	script = strings.ReplaceAll(strings.ReplaceAll(script, "\r", " "), "\n", " ")
	full := "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; " + script
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", full)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("%s", msg)
	}
	return strings.TrimPrefix(out.String(), "\ufeff"), nil
}

func splitOut(s string) []string {
	var res []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

func isAdmin() bool {
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	return exec.Command("fsutil", "dirty", "query", drive).Run() == nil
}

// relaunchAdmin starts an elevated copy and waits for it. Returns false if UAC was declined.
func relaunchAdmin() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe = strings.ReplaceAll(exe, "'", "''")
	err = exec.Command("powershell", "-NoProfile", "-Command", "Start-Process -FilePath '"+exe+"' -Verb RunAs -Wait").Run()
	return err == nil
}

// ---------- registry helpers (reg.exe, no external deps) ----------

var regLine = regexp.MustCompile(`^\s*(.+?)\s{2,}(REG_[A-Z_]+)\s*(.*)$`)

func regGet(key, name string) (string, string, bool) {
	out, err := run("reg", "query", key, "/v", name)
	if err != nil {
		return "", "", false
	}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimRight(l, "\r")
		m := regLine.FindStringSubmatch(l)
		if m != nil && strings.EqualFold(strings.TrimSpace(m[1]), name) {
			return m[2], strings.TrimSpace(m[3]), true
		}
	}
	return "", "", false
}

func regSet(key, name, typ, data string) error {
	out, err := run("reg", "add", key, "/v", name, "/t", typ, "/d", data, "/f")
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(out))
	}
	return nil
}

func regDel(key, name string) {
	run("reg", "delete", key, "/v", name, "/f")
}

func regDelKey(key string) {
	run("reg", "delete", key, "/f")
}

func regKeyExists(key string) bool {
	_, err := run("reg", "query", key)
	return err == nil
}

func parseNum(s string) (uint64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasPrefix(s, "0x") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

func sameRegValue(typ, a, b string) bool {
	if typ == "REG_DWORD" {
		x, e1 := parseNum(a)
		y, e2 := parseNum(b)
		return e1 == nil && e2 == nil && x == y
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// ---------- saved state (exact previous values, used by Restore) ----------

type regPrev struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Existed    bool   `json:"existed"`
	Type       string `json:"type"`
	Data       string `json:"data"`
	KeyExisted bool   `json:"key_existed"`
}

type tweakState struct {
	Applied bool              `json:"applied"`
	Time    string            `json:"time"`
	Regs    []regPrev         `json:"regs"`
	Extra   map[string]string `json:"extra"`
}

type stateFile struct {
	Tweaks map[string]*tweakState `json:"tweaks"`
}

var state *stateFile

func stateDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "FortniteMultiTool")
}

func statePath() string {
	return filepath.Join(stateDir(), "state.json")
}

func loadState() *stateFile {
	if state != nil {
		return state
	}
	state = &stateFile{Tweaks: map[string]*tweakState{}}
	if b, err := os.ReadFile(statePath()); err == nil {
		_ = json.Unmarshal(b, state)
		if state.Tweaks == nil {
			state.Tweaks = map[string]*tweakState{}
		}
	}
	return state
}

func saveState() error {
	b, err := json.MarshalIndent(loadState(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(statePath(), b, 0o644)
}
