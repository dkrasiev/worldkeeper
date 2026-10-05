# Worldkeeper

Automatic backups for Minecraft: Java Edition worlds. Worldkeeper saves a world the moment you exit it in the game and puts the copy in a folder you choose, such as your NAS, an external drive, or a Google Drive / OneDrive / Dropbox folder. It also lets you browse your worlds and load earlier saves, like save slots in other games.

> **Status:** personal project, shared as-is. I use it myself and fix what bothers me. There are no guarantees on issues, feature requests, or response time. PRs are welcome, but they may wait a while.

## Features

- **Finds your worlds** in the official launcher (including custom profile game directories), Prism Launcher, CurseForge, and the Modrinth App on Windows, macOS, and Linux. You can also add any other `saves` folder yourself.
- **Backs up when you exit a world.** Worldkeeper watches each world's `session.lock`, which the game locks while the world is open. When you exit, it saves the world. If the world has not changed since the last backup, it skips the save.
- **Catches up and retries.** On startup it backs up worlds you played while it was not running. If the backup folder is unreachable (NAS asleep, laptop away from home), it retries until it succeeds, and failures show up in the Activity tab.
- **Named saves.** Click "Save now" before fighting the Ender Dragon. Rotation only removes automatic saves and never deletes named ones.
- **Load any save.** You can replace the current world or load the save as a new world next to it. Before replacing, Worldkeeper makes a safety save of the current state, so every restore can be undone.
- **World info read from the files:** version, mode, difficulty, seed, in-game day, weather, spawn, player position, health and level, play time, deaths, mobs killed, distance travelled, advancements, size on disk, explored regions per dimension, data packs, game rules, and mod loaders.
- **Plain zip files.** Every save is an ordinary `.zip` file. If Worldkeeper disappears tomorrow, you can still restore a world with any archive tool.

## Install

Download the archive for your OS from [Releases](https://github.com/dkrasiev/worldkeeper/releases), unpack it, and run `worldkeeper`. Your browser opens the UI at `http://127.0.0.1:25599`.

> The Windows binary is not code-signed, so SmartScreen may warn you the first time you run it. Click "More info" and then "Run anyway", or build it yourself from source.

To start Worldkeeper with your computer:

- **Windows:** put a shortcut to `worldkeeper.exe -no-browser` in `shell:startup`.
- **macOS:** add it under System Settings → General → Login Items.
- **Linux:** use a systemd user service or your desktop's autostart.

## Where backups go

```
<backup folder>/
  minecraft--My_World/
    20261005-183012-auto.zip
    20261005-191000-manual.zip
    index.json
```

Each world gets its own folder, named after the launcher and the world folder. If you reinstall your OS, point Worldkeeper at the same backup folder. Your worlds then show up under **Backups without a local world** and can be restored with one click.

## Limitations

- Java Edition only. Bedrock is not supported.
- The UI is in English.
- Worldkeeper backs up the whole world folder on every save, with no deduplication. Large worlds with many saves take a lot of space, so tune "Automatic saves to keep" in Settings.

## Development

You need Go 1.26+ and Node 24+ (with npm).

```bash
make test     # go vet, go test, TypeScript typecheck
make build    # builds web UI and bin/worldkeeper
```

For UI work, run the Go server and the Vite dev server side by side:

```bash
make dev                  # API on 127.0.0.1:25599
cd web && npm run dev     # open the Vite URL with ?token=<token from config.json>
```

The config lives in your user config directory: `%APPDATA%\worldkeeper\config.json` on Windows, `~/Library/Application Support/worldkeeper/config.json` on macOS, and `~/.config/worldkeeper/config.json` on Linux.

### Layout

| Path | What it does |
|---|---|
| `internal/discovery` | Known launcher paths per OS and the world scan |
| `internal/worldinfo` | Parses `level.dat` (NBT), stats, advancements, and disk usage |
| `internal/lock` | Checks whether the game holds `session.lock` |
| `internal/snapshot` | Zip snapshots, index, rotation, extraction |
| `internal/app` | Backup and restore logic, auto-backup watcher, activity feed |
| `internal/api` | Local HTTP API (loopback only, token protected) and the embedded UI |
| `web/` | React + Vite UI |

Releases are built by GoReleaser when a `v*` tag is pushed.

## License

MIT
