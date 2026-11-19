package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	catDelay   = "delay"
	catNetwork = "network"
	catFPS     = "fps"
)

const (
	sysProfile = `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	ifeoFN     = `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\FortniteClient-Win64-Shipping.exe\PerfOptions`
	fgus       = "[/Script/FortniteGame.FortGameUserSettings]"
	sgs        = "[ScalabilityGroups]"
)

type regEntry struct {
	key, name, typ, data string
	def                  string // value to restore when no saved backup exists
	defAbsent            bool   // true: Windows default is "value does not exist"
	dropKey              bool   // delete the whole key on restore if this tool created it
}

type tweak struct {
	id, cat, name, desc string
	rating              string // Proven, Likely, Marginal, Experimental, Risky
	reboot              bool
	inAll               bool // included in "Apply ALL recommended"
	regs                []regEntry
	apply               func(ts *tweakState) error
	revert              func(ts *tweakState) error
	check               func() string
}

var tweakList = buildTweaks()

func buildTweaks() []*tweak {
	return []*tweak{
		// ───────────────────────── DELAY ─────────────────────────
		{
			id: "mouse-accel", cat: catDelay, rating: "Proven", inAll: true, reboot: true,
			name: "Mouse acceleration off",
			desc: "Turns off 'Enhance pointer precision' so aim is 1:1 (sign out / restart to apply).",
			regs: []regEntry{
				{key: `HKCU\Control Panel\Mouse`, name: "MouseSpeed", typ: "REG_SZ", data: "0", def: "1"},
				{key: `HKCU\Control Panel\Mouse`, name: "MouseThreshold1", typ: "REG_SZ", data: "0", def: "6"},
				{key: `HKCU\Control Panel\Mouse`, name: "MouseThreshold2", typ: "REG_SZ", data: "0", def: "10"},
			},
		},
		{
			id: "sticky-keys", cat: catDelay, rating: "Proven", inAll: true,
			name: "No Sticky/Filter Keys popups",
			desc: "Stops the Shift-spam popup from yanking you out of the game mid-fight.",
			regs: []regEntry{
				{key: `HKCU\Control Panel\Accessibility\StickyKeys`, name: "Flags", typ: "REG_SZ", data: "506", def: "510"},
				{key: `HKCU\Control Panel\Accessibility\Keyboard Response`, name: "Flags", typ: "REG_SZ", data: "122", def: "126"},
				{key: `HKCU\Control Panel\Accessibility\ToggleKeys`, name: "Flags", typ: "REG_SZ", data: "58", def: "62"},
			},
		},
		{
			id: "timer-global", cat: catDelay, rating: "Likely", inAll: true, reboot: true,
			name: "Global timer resolution (Win11)",
			desc: "Makes the game's high-res timer requests apply system-wide again. Pair with Tools → Timer Holder.",
			regs: []regEntry{
				{key: `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\kernel`, name: "GlobalTimerResolutionRequests", typ: "REG_DWORD", data: "1", defAbsent: true},
			},
		},
		{
			id: "mmcss", cat: catDelay, rating: "Marginal", inAll: true,
			name: "MMCSS game profile",
			desc: "Small scheduler nudge for games (SystemResponsiveness 10, Games task High). Safe, tiny effect.",
			regs: []regEntry{
				{key: sysProfile, name: "SystemResponsiveness", typ: "REG_DWORD", data: "10", def: "20"},
				{key: sysProfile + `\Tasks\Games`, name: "GPU Priority", typ: "REG_DWORD", data: "8", def: "8"},
				{key: sysProfile + `\Tasks\Games`, name: "Priority", typ: "REG_DWORD", data: "6", def: "2"},
				{key: sysProfile + `\Tasks\Games`, name: "Scheduling Category", typ: "REG_SZ", data: "High", def: "Medium"},
				{key: sysProfile + `\Tasks\Games`, name: "SFIO Priority", typ: "REG_SZ", data: "High", def: "Normal"},
			},
		},
		{
			id: "fn-fpscap", cat: catDelay, rating: "Likely", inAll: false,
			name: "Fortnite FPS cap (asks a number)",
			desc: "Sets FrameRateLimit in GameUserSettings.ini (can exceed the in-game 240 limit; 0 = uncapped).",
			apply: applyFPSCap, revert: revertIniKeys,
		},
		{
			id: "win32prio", cat: catDelay, rating: "Experimental", inAll: false,
			name: "Foreground priority boost (0x26)",
			desc: "Win32PrioritySeparation=0x26. Evidence is mixed; test it yourself before keeping it.",
			regs: []regEntry{
				{key: `HKLM\SYSTEM\CurrentControlSet\Control\PriorityControl`, name: "Win32PrioritySeparation", typ: "REG_DWORD", data: "38", def: "2"},
			},
		},

		// ───────────────────────── NETWORK ─────────────────────────
		{
			id: "nic-power", cat: catNetwork, rating: "Likely", inAll: true,
			name: "Adapter power-saving off",
			desc: "Turns off EEE / Green Ethernet / flow control / power-saving on your active adapter (brief disconnect).",
			apply: applyNIC, revert: revertNIC,
		},
		{
			id: "do-off", cat: catNetwork, rating: "Marginal", inAll: true,
			name: "Windows Update P2P upload off",
			desc: "Stops Windows sharing updates over your upload bandwidth while you play.",
			regs: []regEntry{
				{key: `HKLM\SOFTWARE\Policies\Microsoft\Windows\DeliveryOptimization`, name: "DODownloadMode", typ: "REG_DWORD", data: "0", defAbsent: true},
			},
		},
		{
			id: "dns", cat: catNetwork, rating: "Marginal", inAll: false,
			name: "Fast DNS (Cloudflare/Google/Quad9)",
			desc: "Speeds up login/matchmaking lookups. Does NOT change in-game ping.",
			apply: applyDNS, revert: revertDNS,
		},
		{
			id: "net-throttle", cat: catNetwork, rating: "Experimental", inAll: false,
			name: "NetworkThrottlingIndex off",
			desc: "Widely shared, no credible benchmark shows a gain on modern Windows. Included for completeness.",
			regs: []regEntry{
				{key: sysProfile, name: "NetworkThrottlingIndex", typ: "REG_DWORD", data: "0xffffffff", def: "10"},
			},
		},

		// ───────────────────────── FPS ─────────────────────────
		{
			id: "power-plan", cat: catFPS, rating: "Likely", inAll: true,
			name: "Ultimate Performance power plan",
			desc: "Stops CPU downclocking mid-fight. Falls back to High Performance if Ultimate is unavailable.",
			apply: applyPower, revert: revertPower, check: powerCheck,
		},
		{
			id: "fn-config", cat: catFPS, rating: "Proven", inAll: true,
			name: "Fortnite competitive config",
			desc: "VSync off, motion blur off, dynamic res off, mouse accel off, low shadows/post/effects/GI/reflections.",
			apply: applyFNConfig, revert: revertIniKeys,
		},
		{
			id: "fn-priority", cat: catFPS, rating: "Likely", inAll: true,
			name: "Fortnite always High priority",
			desc: "Windows launches Fortnite at High CPU + IO priority automatically (never Realtime).",
			regs: []regEntry{
				{key: ifeoFN, name: "CpuPriorityClass", typ: "REG_DWORD", data: "3", defAbsent: true, dropKey: true},
				{key: ifeoFN, name: "IoPriority", typ: "REG_DWORD", data: "3", defAbsent: true, dropKey: true},
			},
		},
		{
			id: "game-dvr", cat: catFPS, rating: "Likely", inAll: true,
			name: "Game DVR / Game Bar capture off",
			desc: "Removes background recording overhead and controller-opens-Game-Bar.",
			regs: []regEntry{
				{key: `HKCU\System\GameConfigStore`, name: "GameDVR_Enabled", typ: "REG_DWORD", data: "0", def: "1"},
				{key: `HKCU\Software\Microsoft\Windows\CurrentVersion\GameDVR`, name: "AppCaptureEnabled", typ: "REG_DWORD", data: "0", def: "1"},
				{key: `HKCU\Software\Microsoft\GameBar`, name: "UseNexusForGameBarEnabled", typ: "REG_DWORD", data: "0", defAbsent: true},
			},
		},
		{
			id: "game-mode", cat: catFPS, rating: "Marginal", inAll: true,
			name: "Windows Game Mode on",
			desc: "Prioritises the game and pauses driver/update interruptions. Small gain on CPU-bound systems.",
			regs: []regEntry{
				{key: `HKCU\Software\Microsoft\GameBar`, name: "AllowAutoGameMode", typ: "REG_DWORD", data: "1", defAbsent: true},
				{key: `HKCU\Software\Microsoft\GameBar`, name: "AutoGameModeEnabled", typ: "REG_DWORD", data: "1", defAbsent: true},
			},
		},
		{
			id: "hags", cat: catFPS, rating: "Experimental", inAll: false, reboot: true,
			name: "Hardware GPU scheduling on",
			desc: "Helps some systems, hurts others. Benchmark with and without.",
			regs: []regEntry{
				{key: `HKLM\SYSTEM\CurrentControlSet\Control\GraphicsDrivers`, name: "HwSchMode", typ: "REG_DWORD", data: "2", defAbsent: true},
			},
		},
		{
			id: "vbs-off", cat: catFPS, rating: "Risky", inAll: false, reboot: true,
			name: "Memory Integrity (HVCI) off",
			desc: "Can recover FPS on older CPUs but weakens Windows security. Only if you accept that.",
			regs: []regEntry{
				{key: `HKLM\SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity`, name: "Enabled", typ: "REG_DWORD", data: "0", defAbsent: true},
			},
		},
	}
}

func tweaksIn(cat string) []*tweak {
	var out []*tweak
	for _, t := range tweakList {
		if t.cat == cat {
			out = append(out, t)
		}
	}
	return out
}

// ---------- status / apply / revert ----------

func tweakStatus(t *tweak) string {
	if t.check != nil {
		return t.check()
	}
	if len(t.regs) > 0 {
		on := 0
		for _, r := range t.regs {
			typ, data, ok := regGet(r.key, r.name)
			if ok && typ == r.typ && sameRegValue(r.typ, data, r.data) {
				on++
			}
		}
		switch {
		case on == len(t.regs):
			return "on"
		case on == 0:
			return "off"
		default:
			return "partial"
		}
	}
	if ts := loadState().Tweaks[t.id]; ts != nil && ts.Applied {
		return "on"
	}
	return "off"
}

var restoreTried bool

func ensureRestorePoint() {
	if restoreTried {
		return
	}
	restoreTried = true
	fmt.Printf("  %sRequesting a Windows restore point...%s\n", gray, reset)
	if _, err := ps(`$ErrorActionPreference='Stop'; Checkpoint-Computer -Description 'Fortnite Multi Tool' -RestorePointType MODIFY_SETTINGS`); err != nil {
		warnMsg("Restore point skipped (System Protection off?). This tool still keeps its own backups.")
	} else {
		okMsg("Restore point step done")
	}
}

func restoreRegs(t *tweak, ts *tweakState) error {
	var firstErr error
	for i, p := range ts.Regs {
		if p.Existed {
			if err := regSet(p.Key, p.Name, p.Type, p.Data); err != nil && firstErr == nil {
				firstErr = err
			}
		} else {
			regDel(p.Key, p.Name)
		}
		if i < len(t.regs) && t.regs[i].dropKey && !p.KeyExisted {
			regDelKey(p.Key)
		}
	}
	return firstErr
}

func applyTweak(t *tweak) error {
	ensureRestorePoint()
	s := loadState()
	old := s.Tweaks[t.id]
	if old != nil && old.Applied && tweakStatus(t) == "on" {
		return nil // already applied; keep the original backup
	}
	ts := old
	fresh := ts == nil || !ts.Applied
	if fresh {
		ts = &tweakState{Extra: map[string]string{}}
		for _, r := range t.regs {
			p := regPrev{Key: r.key, Name: r.name, KeyExisted: regKeyExists(r.key)}
			if typ, data, ok := regGet(r.key, r.name); ok {
				p.Existed, p.Type, p.Data = true, typ, data
			}
			ts.Regs = append(ts.Regs, p)
		}
	}
	if ts.Extra == nil {
		ts.Extra = map[string]string{}
	}
	ts.Time = time.Now().Format(time.RFC3339)
	for _, r := range t.regs {
		if err := regSet(r.key, r.name, r.typ, r.data); err != nil {
			if fresh {
				_ = restoreRegs(t, ts)
			}
			return err
		}
	}
	if t.apply != nil {
		if err := t.apply(ts); err != nil {
			if fresh {
				_ = restoreRegs(t, ts)
			}
			return err
		}
	}
	ts.Applied = true
	s.Tweaks[t.id] = ts
	return saveState()
}

// revertTweak restores the exact saved values. With force=true (or no saved state) it resets to Windows defaults.
func revertTweak(t *tweak, force bool) error {
	s := loadState()
	ts := s.Tweaks[t.id]
	var firstErr error
	if ts != nil && ts.Applied && !force {
		if err := restoreRegs(t, ts); err != nil {
			firstErr = err
		}
		if t.revert != nil {
			if err := t.revert(ts); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	} else {
		for _, r := range t.regs {
			if r.defAbsent {
				regDel(r.key, r.name)
			} else if err := regSet(r.key, r.name, r.typ, r.def); err != nil && firstErr == nil {
				firstErr = err
			}
			if r.dropKey {
				regDelKey(r.key)
			}
		}
		if t.revert != nil {
			if err := t.revert(nil); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr == nil {
		delete(s.Tweaks, t.id)
		return saveState()
	}
	return firstErr
}

// ---------- UI ----------

func ratingTag(r string) string {
	switch r {
	case "Proven":
		return green + "[Proven]" + reset
	case "Likely":
		return green + "[Likely]" + reset
	case "Marginal":
		return yellow + "[Marginal]" + reset
	case "Experimental":
		return magenta + "[Experimental]" + reset
	case "Risky":
		return red + "[Risky]" + reset
	}
	return ""
}

func statusIcon(s string) string {
	switch s {
	case "on":
		return green + "●" + reset
	case "partial":
		return yellow + "◐" + reset
	}
	return gray + "○" + reset
}

func parseNumbers(s string) []int {
	var res []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		if n, err := strconv.Atoi(f); err == nil {
			res = append(res, n)
		}
	}
	return res
}

func adminGate() {
	if isAdmin() {
		return
	}
	clearScreen()
	banner()
	warnMsg("Tweaks need Administrator rights (registry, power plan, network adapter).")
	if !strings.EqualFold(ask("Relaunch as Administrator now? (Y/n): "), "n") {
		if relaunchAdmin() {
			os.Exit(0)
		}
		failMsg("Could not elevate. Right-click the exe and choose Run as administrator.")
		pause()
	}
}

func applyMany(list []*tweak) {
	if len(list) == 0 {
		return
	}
	fmt.Println()
	var rebootNames []string
	for _, t := range list {
		if err := applyTweak(t); err != nil {
			failMsg(t.name + ": " + err.Error())
			continue
		}
		okMsg(t.name)
		if t.reboot {
			rebootNames = append(rebootNames, t.name)
		}
	}
	if len(rebootNames) > 0 {
		warnMsg("Restart Windows to fully apply: " + strings.Join(rebootNames, ", "))
	}
	pause()
}

func revertMany(list []*tweak, force bool) {
	fmt.Println()
	for _, t := range list {
		if err := revertTweak(t, force); err != nil {
			failMsg(t.name + ": " + err.Error())
			continue
		}
		okMsg("Restored: " + t.name)
	}
	pause()
}

func toggleMany(list []*tweak) {
	fmt.Println()
	var rebootNames []string
	for _, t := range list {
		if tweakStatus(t) == "on" {
			if err := revertTweak(t, false); err != nil {
				failMsg(t.name + ": " + err.Error())
			} else {
				okMsg("Restored: " + t.name)
			}
			continue
		}
		if err := applyTweak(t); err != nil {
			failMsg(t.name + ": " + err.Error())
			continue
		}
		okMsg("Applied: " + t.name)
		if t.reboot {
			rebootNames = append(rebootNames, t.name)
		}
	}
	if len(rebootNames) > 0 {
		warnMsg("Restart Windows to fully apply: " + strings.Join(rebootNames, ", "))
	}
	pause()
}

func categoryMenu(cat, title, note string) {
	for {
		clearScreen()
		banner()
		fmt.Printf("  %s%s%s%s\n  %s%s%s\n\n", bold, magenta, title, reset, gray, note, reset)
		list := tweaksIn(cat)
		for i, t := range list {
			fmt.Printf("  %s[%d]%s %s %s%s%s %s\n", cyan, i+1, reset, statusIcon(tweakStatus(t)), bold, t.name, reset, ratingTag(t.rating))
			fmt.Printf("      %s%s%s\n", gray, t.desc, reset)
		}
		fmt.Printf("\n  %s●%s on  %s◐%s partial  %s○%s off     numbers = toggle (e.g. 1 3)  %sa%s = apply recommended  %sr%s = restore this page  %s0%s = back\n\n",
			green, reset, yellow, reset, gray, reset, cyan, reset, cyan, reset, cyan, reset)

		c := strings.ToLower(ask(""))
		switch {
		case c == "0" || c == "":
			return
		case c == "a":
			var sel []*tweak
			for _, t := range list {
				if t.inAll {
					sel = append(sel, t)
				}
			}
			applyMany(sel)
		case c == "r":
			revertMany(list, false)
		default:
			var sel []*tweak
			for _, n := range parseNumbers(c) {
				if n >= 1 && n <= len(list) {
					sel = append(sel, list[n-1])
				}
			}
			if len(sel) > 0 {
				toggleMany(sel)
			}
		}
	}
}

func applyAll() {
	clearScreen()
	banner()
	fmt.Printf("  %s%sAPPLY ALL RECOMMENDED%s\n\n", bold, magenta, reset)
	var sel []*tweak
	for _, t := range tweakList {
		if t.inAll {
			sel = append(sel, t)
			fmt.Printf("  %s•%s %s %s\n", cyan, reset, t.name, ratingTag(t.rating))
		}
	}
	fmt.Printf("\n  %sNot included:%s Experimental/Risky tweaks, DNS and FPS cap (they need your choice).\n", gray, reset)
	fmt.Printf("  %sEverything is backed up first and can be undone from Restore.%s\n\n", gray, reset)
	if !strings.EqualFold(ask("Apply these now? (y/N): "), "y") {
		return
	}
	applyMany(sel)
}

func restoreMenu() {
	for {
		clearScreen()
		banner()
		fmt.Printf("  %s%sRESTORE%s\n\n", bold, magenta, reset)
		menuItem("1", "Undo everything", "Exact previous values (recommended)")
		menuItem("2", "Reset to Windows defaults", "Ignores saved backups")
		menuItem("3", "Where are my backups?", "")
		menuItem("0", "Back", "")
		fmt.Println()
		switch ask("") {
		case "1":
			var sel []*tweak
			for i := len(tweakList) - 1; i >= 0; i-- {
				if ts := loadState().Tweaks[tweakList[i].id]; ts != nil && ts.Applied {
					sel = append(sel, tweakList[i])
				}
			}
			if len(sel) == 0 {
				fmt.Println()
				warnMsg("Nothing to undo — this tool hasn't changed anything yet.")
				pause()
				continue
			}
			revertMany(sel, false)
		case "2":
			fmt.Println()
			warnMsg("This sets every tweak back to the Windows default, even values you had customised before.")
			if !strings.EqualFold(ask("Continue? (y/N): "), "y") {
				continue
			}
			var sel []*tweak
			for i := len(tweakList) - 1; i >= 0; i-- {
				sel = append(sel, tweakList[i])
			}
			revertMany(sel, true)
		case "3":
			fmt.Println()
			fmt.Printf("  %sTweak backups:%s  %s\n", gray, reset, statePath())
			fmt.Printf("  %sConfig backups:%s %s\n", gray, reset, backupDir())
			pause()
		default:
			return
		}
	}
}

func tweaksMenu() {
	adminGate()
	for {
		clearScreen()
		banner()
		fmt.Printf("  %s%sTWEAKS%s  %severy change is backed up and reversible%s\n\n", bold, magenta, reset, gray, reset)
		menuItem("1", "Delay", "Input & timing")
		menuItem("2", "Network", "Adapter, DNS, background traffic")
		menuItem("3", "FPS", "Power, priority, Fortnite config")
		menuItem("4", "Apply ALL recommended", "One go, fully backed up")
		menuItem("5", "Tools", "System check, timer holder, cleanup")
		menuItem("6", "Restore", "Undo everything")
		menuItem("0", "Back", "")
		fmt.Printf("\n  %sRatings: Proven = measured/consistent · Likely = sound but system-dependent · Marginal = tiny · Experimental/Risky = opt-in%s\n", gray, reset)
		fmt.Printf("  %sNagle/TcpAckFrequency tweaks are intentionally left out: Fortnite uses UDP, so they do nothing for it.%s\n\n", gray, reset)

		switch ask("") {
		case "1":
			categoryMenu(catDelay, "DELAY", "Input lag & timing precision")
		case "2":
			categoryMenu(catNetwork, "NETWORK", "Jitter, background traffic, lookups")
		case "3":
			categoryMenu(catFPS, "FPS", "Frame rate & frame pacing")
		case "4":
			applyAll()
		case "5":
			toolsMenu()
		case "6":
			restoreMenu()
		default:
			return
		}
	}
}
