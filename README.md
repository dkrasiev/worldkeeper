# Worldkeeper

Automatic backups for Minecraft: Java Edition worlds. Worldkeeper saves a world the moment you exit it in the game and puts the copy in a folder you choose, such as your NAS, an external drive, or a Google Drive / OneDrive / Dropbox folder. It also lets you browse your worlds and load earlier saves, like save slots in other games.

> **Status:** personal project, shared as-is. I use it myself and fix what bothers me. There are no guarantees on issues, feature requests, or response time. PRs are welcome, but they may wait a while.

## Features

- **Finds your worlds** in the official launcher (including custom profile game directories), Prism Launcher, CurseForge, and the Modrinth App on Windows, macOS, and Linux. You can also add any other `saves` folder yourself.
- **Backs up when you exit a world.** Worldkeeper watches each world's `session.lock`, which the game locks while the world is open. When you exit, it saves the world. If the world has not changed since the last backup, it skips the save.
- **Catches up and retries.** On startup it backs up worlds you played while it was not running. If the backup folder is unreachable (NAS asleep, laptop away from home), it retries until it succeeds, and failures show up in the Activity tab.
- **Named saves.** Click "Save now" before fighting the Ender Dragon. Rotation only removes automatic saves and never deletes named ones.
- **Load any save.** You can replace the current world or load the save as a new world next to it, named "… (restored <date>)" so the game lists both. Before replacing, Worldkeeper makes a safety save of the current state, so every restore can be undone.
- **World info read from the files:** version, mode, difficulty, seed, in-game day, weather, spawn, player position, health and level, play time, deaths, mobs killed, distance travelled, advancements, size on disk, explored regions per dimension, data packs, game rules, and mod loaders.
- **Advancement tree.** The world page can show the in-game advancement screen with the player's progress, drawn by [mcwidgets](https://github.com/KabanFriends/mcwidgets). It is off until you click **Show advancement tree**: the widget loads from `mcwidgets.kaban.sh` in a sandboxed iframe, so it needs internet access, and the world's advancement progress is sent to that site. Only vanilla advancements are shown. Worldkeeper does not ship Minecraft textures itself.
- **Plain zip files or a restic repository.** By default every save is an ordinary `.zip` file, so you can restore a world with any archive tool even without Worldkeeper. Alternatively, saves can go to a [restic](https://restic.net) repository: encrypted and deduplicated, so dozens of saves of a big world take little extra space.

See the [roadmap](ROADMAP.md) for what is planned next.

## Install

Download the archive for your OS from [Releases](https://github.com/dkrasiev/worldkeeper/releases), unpack it, and run `worldkeeper`. On the first start your browser opens the UI at `http://127.0.0.1:25599`.

> The Windows binary is not code-signed, so SmartScreen may warn you the first time you run it. Click "More info" and then "Run anyway", or build it yourself from source.

### Tray icon

While running, Worldkeeper sits in the system tray (Windows), the menu bar (macOS), or the status area (Linux). Its menu:

- shows when the last backup happened, or which world failed. The icon gets a red dot while a backup is failing;
- **Open Worldkeeper** opens the web UI;
- **Back up now** backs up every world that changed since its last save;
- **Back up when a world is closed** toggles the automatic backup;
- **Quit Worldkeeper** stops it.

The menu follows the system language (English or Russian). With the tray available, the browser opens on its own only on the very first start. Pass `-no-tray` to run without the icon, for example on a server. On Linux the icon needs a desktop with StatusNotifier support (KDE, or GNOME with the AppIndicator extension), and Worldkeeper runs without it when there is no D-Bus session.

The log is written next to the config, in `worldkeeper.log`. The Activity feed (the last 100 events) is kept there too, in `events.json`, so failures from before a restart stay visible.

To start Worldkeeper with your computer:

- **Windows:** put a shortcut to `worldkeeper.exe` in `shell:startup`.
- **macOS:** add it under System Settings → General → Login Items.
- **Linux:** use a systemd user service or your desktop's autostart.

## Restic storage (optional)

1. Install restic 0.17 or newer: `winget install restic.restic` (Windows), `brew install restic` (macOS), or your package manager.
2. In **Settings**, choose **Restic repository**. Enter the repository (a folder or network share, `sftp:user@host:/path`, `rest:http://...`, and so on) and a password.
3. Click **Save settings**. If nothing is stored there yet, click **Initialize repository**.

The password is kept in the system keychain (Keychain, Windows Credential Manager, or Secret Service), never in the config file. On Linux without a Secret Service, use **Advanced → Password file**. **If you lose the password, the backups cannot be restored.**

Each save is a restic snapshot tagged `wk`, `world:<id>`, and `kind:auto|manual|pre-restore`, so you can work with it using plain restic too:

```bash
restic -r <repo> snapshots --tag wk
restic -r <repo> restore <snapshot-id> --target ./restored-world
```

Worlds are backed up from inside their folder, so a restore puts the world files directly into the target folder.

## Where zip backups go

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
- Switching between zip and restic does not migrate existing saves. Each storage keeps its own saves.
- With zip storage, every save contains the whole world. Large worlds with many saves take a lot of space, so use restic storage or lower "Automatic saves to keep" in Settings.

## Development

You need Go 1.26+, Node 24+, and pnpm. On macOS the menu bar icon needs cgo (Xcode command line tools). Without cgo, macOS builds run without the icon.

```bash
make test     # go vet, go test, TypeScript typecheck
make build    # builds web UI and bin/worldkeeper
```

For UI work, run the Go server and the Vite dev server side by side:

```bash
make dev                  # API on 127.0.0.1:25599
cd web && pnpm dev        # open the Vite URL with ?token=<token from config.json>
```

The config lives in your user config directory: `%APPDATA%\worldkeeper\config.json` on Windows, `~/Library/Application Support/worldkeeper/config.json` on macOS, and `~/.config/worldkeeper/config.json` on Linux.

### Translations

The UI is available in English and Russian. It follows the browser language, and you can switch it in the top bar. To add a language:

1. Copy `web/src/i18n/ru.ts` to `web/src/i18n/<code>.ts` and translate the values. TypeScript fails the build if a key is missing.
2. Register it in `web/src/i18n/index.tsx` (`dictionaries` and `LOCALES`).

Plural entries use the CLDR categories (`one`, `few`, `many`, `other`, …) that your language needs. Dates, sizes, and durations are formatted with `Intl` for the selected language. The server sends events and errors as codes with parameters, and the UI renders them in the selected language.

### Layout

| Path | What it does |
|---|---|
| `internal/discovery` | Known launcher paths per OS and the world scan |
| `internal/worldinfo` | Parses `level.dat` (NBT), stats, advancements, and disk usage |
| `internal/lock` | Checks whether the game holds `session.lock` |
| `internal/snapshot` | Storage backend interface; zip snapshots, index, rotation, extraction |
| `internal/restic` | Restic backend: runs the `restic` CLI, maps snapshots to tags |
| `internal/secrets` | Keeps the restic password in the OS credential store |
| `internal/app` | Backup and restore logic, auto-backup watcher, activity feed |
| `internal/api` | Local HTTP API (loopback only, token protected) and the embedded UI |
| `web/` | React + Vite UI; translations in `web/src/i18n/` |

**Dev builds.** Every pull request and every push to `main` gets a Windows x64 test build (`worldkeeper.exe`, versioned like `0.2.0-dev.abc1234`, after the next minor release), made once per commit. To get one, open the *Dev build* run for that commit in the [Actions tab](https://github.com/dkrasiev/worldkeeper/actions/workflows/dev-build.yml) and download it from *Artifacts*. You need to be signed in to GitHub, and artifacts are kept for 14 days. Dev builds are for testing and are not releases.

### Pull requests and commits

Pull requests are squash-merged: each one becomes a single commit on `main`, titled after the pull request. So the pull request title follows [Conventional Commits](https://www.conventionalcommits.org/): `type: what changed`, e.g. `feat: show progress for long backups` or `fix: no console window for restic on Windows`. Commits inside a pull request can be anything.

| Type | For | In release notes |
|---|---|---|
| `feat` | new features | New features |
| `fix`, `perf` | bug fixes, speedups | Bug fixes |
| `build` | packaging, installers | Other changes |
| `docs`, `test`, `ci`, `chore`, `refactor` | everything users do not see | left out |

### Releasing

Pushing a `v*` tag builds the release: GoReleaser makes the archives and the GitHub release, and opens the winget pull request if the `WINGET_TOKEN` secret is set. The tag can be on any commit.

**Regular release** from `main`:

```bash
git switch main && git pull
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

**Hotfix** for a past release, when `main` already has unreleased work. Merge the fix into `main` first, then:

```bash
git switch -c release/v0.1 v0.1.0   # or reuse release/v0.1 from an earlier hotfix
git cherry-pick <fix-commit>
git push -u origin release/v0.1     # CI runs on release/* branches
git tag -a v0.1.1 -m "v0.1.1"
git push origin v0.1.1
```

Keep `release/v0.1` for later hotfixes (`v0.1.2`, …). If `main` has nothing unreleased, tag `main` instead.

## License

MIT
