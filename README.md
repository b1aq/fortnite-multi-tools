# Fortnite Multi Tool
(i use ai)

A colorful Windows terminal toolbox for Fortnite. Single `.exe`, no install, no dependencies.

## Tools

| # | Tool | What it does |
|---|------|--------------|
| 1 | Live Ping | Pings every matchmaking region at once (NAE, NAC, NAW, EU, BR, ASIA, ME, OCE) in real time: ping, avg, min, max, jitter, loss |
| 2 | Stretched Resolution | Writes a stretched resolution into `GameUserSettings.ini` (auto backup first) |
| 3 | Restore Config Backup | Rolls `GameUserSettings.ini` back to a previous backup |
| 2 | Tweaks | Delay · Network · FPS tweaks with per-tweak ratings, status, full backup and one-click restore |

### Tweaks

Every tweak is rated honestly: **Proven**, **Likely**, **Marginal**, **Experimental**, **Risky**. "Apply ALL recommended" only applies Proven/Likely/Marginal ones that need no input.
The exact previous values are saved to `%LOCALAPPDATA%\FortniteMultiTool\state.json` so **Restore → Undo everything** puts things back precisely. A Windows restore point is requested before the first change.

Intentionally not included: Nagle / TcpAckFrequency (Fortnite uses UDP, so they do nothing for it).

Run as Administrator for tweaks (the tool offers to relaunch itself).

## Download

Grab `fortnite-multi-tool.exe` from the [Releases](../../releases) page and run it.

## Build / release

Builds run on GitHub Actions (Windows runner, Go). To publish a release, push a tag:

```
git tag v0.1.0
git push origin v0.1.0
```

Or create a release with a new `v*` tag in the GitHub UI. Manual runs (Actions → Run workflow) upload the exe as a build artifact.

## Notes

- Ping is TCP connect latency to nearby AWS regional endpoints. It's close to in-game ping, not identical.
- Close Fortnite before changing resolution, otherwise the game overwrites the file.
- This tool only edits your own local config file. It does not touch the game process or memory.

## License

MIT
