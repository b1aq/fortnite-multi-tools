# Fortnite Multi Tool

A colorful Windows terminal toolbox for Fortnite. Single `.exe`, no install, no dependencies.

## Tools

| # | Tool | What it does |
|---|------|--------------|
| 1 | Live Ping | Pings every matchmaking region at once (NAE, NAC, NAW, EU, BR, ASIA, ME, OCE) in real time: ping, avg, min, max, jitter, loss |
| 2 | Stretched Resolution | Writes a stretched resolution into `GameUserSettings.ini` (auto backup first) |
| 3 | Restore Backup | Rolls `GameUserSettings.ini` back to a previous backup |

More tools (tweaks, etc.) are planned.

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
