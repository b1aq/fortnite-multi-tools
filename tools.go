package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func toolsMenu() {
	for {
		clearScreen()
		banner()
		fmt.Printf("  %s%sTOOLS%s\n\n", bold, magenta, reset)
		menuItem("1", "System check", "Read-only report of your setup")
		menuItem("2", "Timer holder", "Hold 0.5 ms timer while you play")
		menuItem("3", "Clean Fortnite junk", "Logs, crash dumps, webcache")
		menuItem("4", "Flush DNS", "")
		menuItem("0", "Back", "")
		fmt.Println()
		switch ask("") {
		case "1":
			systemCheck()
		case "2":
			timerTool()
		case "3":
			cleanTool()
		case "4":
			fmt.Println()
			if _, err := run("ipconfig", "/flushdns"); err != nil {
				failMsg("Flush failed: " + err.Error())
			} else {
				okMsg("DNS cache flushed")
			}
			pause()
		default:
			return
		}
	}
}

func systemCheck() {
	clearScreen()
	banner()
	fmt.Printf("  %s%sSYSTEM CHECK%s\n\n", bold, magenta, reset)
	row := func(label, value, color string) {
		fmt.Printf("  %-22s %s%s%s\n", label, color, value, reset)
	}

	if isAdmin() {
		row("Administrator", "yes", green)
	} else {
		row("Administrator", "no (tweaks need it)", yellow)
	}
	if powerCheck() == "on" {
		row("Power plan", "High / Ultimate performance", green)
	} else {
		row("Power plan", "not high-performance", yellow)
	}
	if _, d, ok := regGet(`HKCU\Software\Microsoft\GameBar`, "AutoGameModeEnabled"); ok && sameRegValue("REG_DWORD", d, "0") {
		row("Game Mode", "off", yellow)
	} else {
		row("Game Mode", "on", green)
	}
	if _, d, ok := regGet(`HKCU\System\GameConfigStore`, "GameDVR_Enabled"); ok && sameRegValue("REG_DWORD", d, "0") {
		row("Game DVR capture", "off", green)
	} else {
		row("Game DVR capture", "on (adds overhead)", yellow)
	}
	if _, d, ok := regGet(`HKLM\SYSTEM\CurrentControlSet\Control\GraphicsDrivers`, "HwSchMode"); !ok {
		row("HW GPU scheduling", "driver default", gray)
	} else if sameRegValue("REG_DWORD", d, "2") {
		row("HW GPU scheduling", "on", green)
	} else {
		row("HW GPU scheduling", "off", yellow)
	}
	if _, d, ok := regGet(`HKLM\SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity`, "Enabled"); ok && sameRegValue("REG_DWORD", d, "1") {
		row("Memory Integrity", "on (secure, can cost FPS)", yellow)
	} else {
		row("Memory Integrity", "off", gray)
	}
	if _, d, ok := regGet(`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\kernel`, "GlobalTimerResolutionRequests"); ok && sameRegValue("REG_DWORD", d, "1") {
		row("Global timer tweak", "on", green)
	} else {
		row("Global timer tweak", "off", gray)
	}
	if coarse, fine, cur, ok := queryTimer(); ok {
		row("Timer resolution", fmt.Sprintf("now %.3f ms (finest %.3f, default %.3f)", float64(cur)/10000, float64(fine)/10000, float64(coarse)/10000), gray)
	}

	fmt.Println()
	out, err := ps(`Get-NetAdapter -Physical | Where-Object { $_.Status -eq 'Up' } | ForEach-Object { ('{0}|{1}|{2}' -f $_.Name,$_.LinkSpeed,$_.PhysicalMediaType) }`)
	if err != nil {
		row("Network adapters", "could not read", yellow)
	} else {
		for _, l := range splitOut(out) {
			p := strings.Split(l, "|")
			if len(p) != 3 {
				continue
			}
			wifi := strings.Contains(strings.ToLower(p[2]), "802.11") || strings.Contains(strings.ToLower(p[2]), "wireless")
			if wifi {
				row("Adapter", p[0]+" · "+p[1]+" · Wi-Fi — Ethernet gives lower jitter", yellow)
			} else {
				row("Adapter", p[0]+" · "+p[1]+" · wired", green)
			}
		}
	}

	fmt.Println()
	path := settingsPath()
	if data, err := os.ReadFile(path); err == nil {
		lines, _ := splitLines(string(data))
		get := func(k string) string {
			if v, ok := iniGet(lines, fgus, k); ok {
				return v
			}
			return "?"
		}
		row("Fortnite resolution", get("ResolutionSizeX")+"x"+get("ResolutionSizeY"), gray)
		row("Fortnite VSync", get("bUseVSync"), gray)
		row("Fortnite FPS limit", get("FrameRateLimit"), gray)
	} else {
		row("Fortnite config", "not found (launch the game once)", yellow)
	}

	fmt.Printf("\n  %sIn Fortnite: set NVIDIA Reflex Low Latency to On + Boost and Rendering Mode to Performance — the two biggest latency wins.%s\n", gray, reset)
	pause()
}

func timerTool() {
	clearScreen()
	banner()
	fmt.Printf("  %s%sTIMER HOLDER%s\n\n", bold, magenta, reset)
	coarse, fine, cur, ok := queryTimer()
	if !ok {
		failMsg("Timer API is not available on this system.")
		pause()
		return
	}
	ms := func(v uint32) float64 { return float64(v) / 10000 }
	fmt.Printf("  Default %.3f ms · finest %.3f ms · right now %.3f ms\n\n", ms(coarse), ms(fine), ms(cur))
	got, ok := setTimer(fine, true)
	if !ok {
		failMsg("Could not request a finer timer.")
		pause()
		return
	}
	okMsg(fmt.Sprintf("Holding %.3f ms. Keep this window open while you play.", ms(got)))
	fmt.Printf("  %sOn Windows 11 this only helps the game system-wide if 'Global timer resolution' is applied (Tweaks → Delay) and you restarted.%s\n", gray, reset)
	fmt.Printf("  %sPress Enter to release.%s\n", gray, reset)
	readLine()
	setTimer(fine, false)
}

func dirSize(root string) int64 {
	var n int64
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func humanSize(n int64) string {
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	units := []string{"KB", "MB", "GB"}
	i := -1
	for f >= u && i < len(units)-1 {
		f /= u
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func cleanTool() {
	clearScreen()
	banner()
	fmt.Printf("  %s%sCLEAN FORTNITE JUNK%s\n\n", bold, magenta, reset)
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		failMsg("LOCALAPPDATA is not set (Windows only).")
		pause()
		return
	}
	saved := filepath.Join(base, "FortniteGame", "Saved")
	targets := []string{filepath.Join(saved, "Logs"), filepath.Join(saved, "Crashes")}
	if m, err := filepath.Glob(filepath.Join(saved, "webcache*")); err == nil {
		targets = append(targets, m...)
	}
	var total int64
	for _, t := range targets {
		s := dirSize(t)
		total += s
		fmt.Printf("  %s%-70s%s %s\n", gray, t, reset, humanSize(s))
	}
	if total == 0 {
		okMsg("Nothing to clean.")
		pause()
		return
	}
	fmt.Printf("\n  Total: %s%s%s\n", bold, humanSize(total), reset)
	if fortniteRunning() {
		warnMsg("Fortnite is running — close it first for a full clean.")
	}
	if !strings.EqualFold(ask("Delete these? (y/N): "), "y") {
		return
	}
	for _, t := range targets {
		entries, err := os.ReadDir(t)
		if err != nil {
			continue
		}
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(t, e.Name()))
		}
	}
	okMsg("Cleaned " + humanSize(total))
	pause()
}
