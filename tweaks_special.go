package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	guidRe    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reNum     = regexp.MustCompile(`^[0-9]+$`)
	reKeyword = regexp.MustCompile(`^[A-Za-z0-9*_]+$`)
	reAddr    = regexp.MustCompile(`^[0-9a-fA-F:.]+$`)
)

// ───────────────────────── Power plan ─────────────────────────

const (
	ourPlan     = "4f4e4c01-2f3d-4b6a-9a3e-6d756c746901"
	highPerf    = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	balancedGUID = "381b4222-f694-41f0-9685-ff5bb260df2e"
	ultimateSrc = "e9a42b02-d5df-448d-aa00-03f14749eb61"
)

func activeScheme() string {
	out, err := run("powercfg", "/getactivescheme")
	if err != nil {
		return ""
	}
	return strings.ToLower(guidRe.FindString(out))
}

func powerCheck() string {
	a := activeScheme()
	if a == ourPlan || a == highPerf || a == ultimateSrc {
		return "on"
	}
	return "off"
}

func applyPower(ts *tweakState) error {
	if _, ok := ts.Extra["prev"]; !ok {
		ts.Extra["prev"] = activeScheme()
	}
	target := ""
	list, _ := run("powercfg", "/list")
	if strings.Contains(strings.ToLower(list), ourPlan) {
		target = ourPlan
	} else if _, err := run("powercfg", "-duplicatescheme", ultimateSrc, ourPlan); err == nil {
		run("powercfg", "/changename", ourPlan, "Fortnite Multi Tool - Ultimate Performance")
		target = ourPlan
	} else {
		target = highPerf
	}
	if out, err := run("powercfg", "/setactive", target); err != nil {
		return fmt.Errorf("powercfg: %s", strings.TrimSpace(out))
	}
	return nil
}

func revertPower(ts *tweakState) error {
	prev := balancedGUID
	if ts != nil && ts.Extra != nil && ts.Extra["prev"] != "" {
		prev = ts.Extra["prev"]
	}
	if out, err := run("powercfg", "/setactive", prev); err != nil {
		return fmt.Errorf("powercfg: %s", strings.TrimSpace(out))
	}
	if prev != ourPlan {
		run("powercfg", "/delete", ourPlan)
	}
	return nil
}

// ───────────────────────── Network adapter ─────────────────────────

const nicApplyScript = `$kw=@('*EEE','EEELinkAdvertisement','AdvancedEEE','EnableGreenEthernet','GreenEthernet','PowerSavingMode','ULPMode','*FlowControl');
Get-NetAdapter -Physical | Where-Object { $_.Status -eq 'Up' } | ForEach-Object {
  $a=$_; $changed=$false;
  foreach($k in $kw){
    $p=Get-NetAdapterAdvancedProperty -Name $a.Name -RegistryKeyword $k -ErrorAction SilentlyContinue;
    if($p -and $p.RegistryValue -and ([string]$p.RegistryValue[0]) -ne '0'){
      $old=[string]$p.RegistryValue[0];
      try { Set-NetAdapterAdvancedProperty -Name $a.Name -RegistryKeyword $k -RegistryValue 0 -NoRestart -ErrorAction Stop; ('{0}|{1}|{2}' -f $a.ifIndex,$k,$old); $changed=$true } catch {}
    }
  }
  if($changed){ Restart-NetAdapter -Name $a.Name -ErrorAction SilentlyContinue }
}`

func applyNIC(ts *tweakState) error {
	if _, ok := ts.Extra["nic"]; ok {
		return nil
	}
	fmt.Printf("  %sApplying to your active adapter — you may disconnect for a few seconds...%s\n", gray, reset)
	out, err := ps(nicApplyScript)
	if err != nil {
		return err
	}
	var items []string
	for _, l := range splitOut(out) {
		if strings.Count(l, "|") == 2 {
			items = append(items, l)
		}
	}
	if len(items) == 0 {
		return fmt.Errorf("no power-saving options to change on your adapter (already off, or not exposed by the driver)")
	}
	ts.Extra["nic"] = strings.Join(items, ";")
	return nil
}

func revertNIC(ts *tweakState) error {
	if ts == nil || ts.Extra == nil || ts.Extra["nic"] == "" {
		return nil
	}
	var cmds []string
	seen := map[string]bool{}
	for _, item := range strings.Split(ts.Extra["nic"], ";") {
		p := strings.Split(item, "|")
		if len(p) != 3 || !reNum.MatchString(p[0]) || !reKeyword.MatchString(p[1]) || !reNum.MatchString(p[2]) {
			continue
		}
		cmds = append(cmds, fmt.Sprintf("Set-NetAdapterAdvancedProperty -Name (Get-NetAdapter -InterfaceIndex %s).Name -RegistryKeyword '%s' -RegistryValue %s -NoRestart -ErrorAction SilentlyContinue;", p[0], p[1], p[2]))
		seen[p[0]] = true
	}
	for idx := range seen {
		cmds = append(cmds, fmt.Sprintf("Restart-NetAdapter -Name (Get-NetAdapter -InterfaceIndex %s).Name -ErrorAction SilentlyContinue;", idx))
	}
	if len(cmds) == 0 {
		return nil
	}
	_, err := ps(strings.Join(cmds, " "))
	return err
}

// ───────────────────────── DNS ─────────────────────────

const dnsQueryScript = `Get-NetAdapter -Physical | Where-Object { $_.Status -eq 'Up' } | ForEach-Object { $ns=''; try { $ns=(Get-ItemProperty -Path ('HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\'+$_.InterfaceGuid) -ErrorAction Stop).NameServer } catch {}; ('{0}|{1}' -f $_.ifIndex,$ns) }`

func applyDNS(ts *tweakState) error {
	fmt.Println()
	fmt.Printf("  %s[1]%s Cloudflare 1.1.1.1    %s[2]%s Google 8.8.8.8    %s[3]%s Quad9 9.9.9.9\n", cyan, reset, cyan, reset, cyan, reset)
	var a, b string
	switch ask("Provider: ") {
	case "1":
		a, b = "1.1.1.1", "1.0.0.1"
	case "2":
		a, b = "8.8.8.8", "8.8.4.4"
	case "3":
		a, b = "9.9.9.9", "149.112.112.112"
	default:
		return fmt.Errorf("cancelled")
	}
	if _, ok := ts.Extra["dns"]; !ok {
		out, err := ps(dnsQueryScript)
		if err != nil {
			return err
		}
		var items []string
		for _, l := range splitOut(out) {
			if strings.Contains(l, "|") {
				items = append(items, l)
			}
		}
		ts.Extra["dns"] = strings.Join(items, ";")
	}
	var cmds []string
	for _, it := range strings.Split(ts.Extra["dns"], ";") {
		idx := strings.SplitN(it, "|", 2)[0]
		if reNum.MatchString(idx) {
			cmds = append(cmds, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %s -ServerAddresses ('%s','%s');", idx, a, b))
		}
	}
	if len(cmds) == 0 {
		delete(ts.Extra, "dns")
		return fmt.Errorf("no active network adapter found")
	}
	if _, err := ps(strings.Join(cmds, " ") + " ipconfig /flushdns | Out-Null"); err != nil {
		return err
	}
	return nil
}

func revertDNS(ts *tweakState) error {
	var cmds []string
	if ts != nil && ts.Extra != nil && ts.Extra["dns"] != "" {
		for _, it := range strings.Split(ts.Extra["dns"], ";") {
			parts := strings.SplitN(it, "|", 2)
			if len(parts) != 2 || !reNum.MatchString(parts[0]) {
				continue
			}
			addrs := strings.FieldsFunc(parts[1], func(r rune) bool { return r == ',' || r == ' ' })
			ok := len(addrs) > 0
			var quoted []string
			for _, ad := range addrs {
				if !reAddr.MatchString(ad) {
					ok = false
					break
				}
				quoted = append(quoted, "'"+ad+"'")
			}
			if ok {
				cmds = append(cmds, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %s -ServerAddresses (%s);", parts[0], strings.Join(quoted, ",")))
			} else {
				cmds = append(cmds, fmt.Sprintf("Set-DnsClientServerAddress -InterfaceIndex %s -ResetServerAddresses;", parts[0]))
			}
		}
	}
	if len(cmds) == 0 {
		cmds = append(cmds, "Get-NetAdapter -Physical | Where-Object { $_.Status -eq 'Up' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex -ResetServerAddresses };")
	}
	_, err := ps(strings.Join(cmds, " ") + " ipconfig /flushdns | Out-Null")
	return err
}

// ───────────────────────── Fortnite GameUserSettings.ini ─────────────────────────

type iniEntry struct{ sec, key, val string }

func iniRange(lines []string, sec string) (int, int) {
	start := -1
	for i, l := range lines {
		if strings.EqualFold(strings.TrimSpace(l), sec) {
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

func iniFind(lines []string, start, end int, key string) int {
	for i := start + 1; i < end; i++ {
		kv := strings.SplitN(lines[i], "=", 2)
		if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), key) {
			return i
		}
	}
	return -1
}

func iniGet(lines []string, sec, key string) (string, bool) {
	start, end := iniRange(lines, sec)
	if start == -1 {
		return "", false
	}
	i := iniFind(lines, start, end, key)
	if i == -1 {
		return "", false
	}
	return strings.TrimSpace(strings.SplitN(lines[i], "=", 2)[1]), true
}

func iniSet(lines []string, sec, key, val string) []string {
	start, end := iniRange(lines, sec)
	if start == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		return append(lines, sec, key+"="+val)
	}
	if i := iniFind(lines, start, end, key); i != -1 {
		lines[i] = key + "=" + val
		return lines
	}
	ins := start + 1
	tail := append([]string{key + "=" + val}, lines[ins:]...)
	return append(lines[:ins], tail...)
}

func iniDel(lines []string, sec, key string) []string {
	start, end := iniRange(lines, sec)
	if start == -1 {
		return lines
	}
	i := iniFind(lines, start, end, key)
	if i == -1 {
		return lines
	}
	return append(lines[:i], lines[i+1:]...)
}

func writeSettings(path string, lines []string, crlf bool) error {
	wasRO := false
	if fi, err := os.Stat(path); err == nil {
		wasRO = fi.Mode().Perm()&0o200 == 0
	}
	_ = os.Chmod(path, 0o666)
	if err := os.WriteFile(path, []byte(joinLines(lines, crlf)), 0o666); err != nil {
		return err
	}
	if wasRO {
		_ = os.Chmod(path, 0o444)
	}
	return nil
}

func applyIniKeys(ts *tweakState, entries []iniEntry) error {
	path := settingsPath()
	if path == "" {
		return fmt.Errorf("LOCALAPPDATA is not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("GameUserSettings.ini not found — launch Fortnite once first")
	}
	if fortniteRunning() {
		warnMsg("Fortnite is running — close it, or it will overwrite these settings on exit.")
	}
	lines, crlf := splitLines(string(data))
	for _, e := range entries {
		k := "ini|" + e.sec + "|" + e.key
		if _, seen := ts.Extra[k]; !seen {
			if v, ok := iniGet(lines, e.sec, e.key); ok {
				ts.Extra[k] = "=" + v
			} else {
				ts.Extra[k] = "-"
			}
		}
		lines = iniSet(lines, e.sec, e.key, e.val)
	}
	if _, err := backupFile(data); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}
	return writeSettings(path, lines, crlf)
}

func revertIniKeys(ts *tweakState) error {
	if ts == nil || ts.Extra == nil {
		return nil
	}
	path := settingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines, crlf := splitLines(string(data))
	changed := false
	for k, v := range ts.Extra {
		if !strings.HasPrefix(k, "ini|") {
			continue
		}
		parts := strings.SplitN(k, "|", 3)
		if len(parts) != 3 {
			continue
		}
		if v == "-" {
			lines = iniDel(lines, parts[1], parts[2])
		} else {
			lines = iniSet(lines, parts[1], parts[2], strings.TrimPrefix(v, "="))
		}
		changed = true
	}
	if !changed {
		return nil
	}
	if _, err := backupFile(data); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}
	return writeSettings(path, lines, crlf)
}

func applyFNConfig(ts *tweakState) error {
	return applyIniKeys(ts, []iniEntry{
		{fgus, "bUseVSync", "False"},
		{fgus, "bMotionBlur", "False"},
		{fgus, "bUseDynamicResolution", "False"},
		{fgus, "bDisableMouseAcceleration", "True"},
		{sgs, "sg.ShadowQuality", "0"},
		{sgs, "sg.PostProcessQuality", "0"},
		{sgs, "sg.EffectsQuality", "0"},
		{sgs, "sg.GlobalIlluminationQuality", "0"},
		{sgs, "sg.ReflectionQuality", "0"},
	})
}

func refreshHint() string {
	out, err := ps(`(Get-CimInstance Win32_VideoController | Where-Object { $_.CurrentRefreshRate } | Select-Object -First 1).CurrentRefreshRate`)
	if err != nil {
		return ""
	}
	hz, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil || hz < 30 {
		return ""
	}
	return fmt.Sprintf(" — display is %d Hz, suggested %d", hz, hz-3)
}

func applyFPSCap(ts *tweakState) error {
	v := ask("FPS limit (0 = uncapped)" + refreshHint() + ": ")
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || n > 1000 {
		return fmt.Errorf("invalid number")
	}
	return applyIniKeys(ts, []iniEntry{{fgus, "FrameRateLimit", fmt.Sprintf("%d.000000", n)}})
}
