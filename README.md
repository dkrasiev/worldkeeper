# Worldkeeper

Automatic backups for Minecraft: Java Edition worlds. Worldkeeper saves a world the moment you exit it in the game and puts the copy in a folder you choose, such as your NAS, an external drive, or a Google Drive / OneDrive / Dropbox folder. It also lets you browse your worlds and load earlier saves, like save slots in other games.

> **Status:** personal project, shared as-is. I use it myself and fix what bothers me. There are no guarantees on issues, feature requests, or response time. PRs are welcome, but they may wait a while.

## Features

- **Finds your worlds** in the official launcher (including custom profile game directories), Prism Launcher, CurseForge, and the Modrinth App on Windows, macOS, and Linux. You can also add any other `saves` folder yourself.
- **Backs up when you exit a world.** Worldkeeper watches each world's `session.lock`, which the game locks while the world is open. When you exit, it saves the world. If the world has not changed since the last backup, it skips the save.
- **Catches up and retries.** On startup it backs up worlds you played while it was not running. If the backup folder is unreachable (NAS asleep, laptop away from home), it retries until it succeeds. Failures show up in the Activity tab and as a desktop notification.
- **Named saves.** Click "Save now" before fighting the Ender Dragon. Rotation only removes automatic saves and never deletes named ones.
- **Keeps yesterday, last week and last month.** Besides the newest automatic saves, Worldkeeper can keep one save per day, week and month, so an old state is still there after a long play session. See [Which automatic saves are kept](#which-automatic-saves-are-kept).
- **Load any save.** You can replace the current world or load the save as a new world next to it, named "… (restored <date>)" so the game lists both. Before replacing, Worldkeeper makes a safety save of the current state, so every restore can be undone.
- **Rename and delete worlds.** Renaming changes the name shown in the game; the folder keeps its name, so the world stays linked to its saves. Before deleting, Worldkeeper saves the world unless its latest state is already saved, so a deleted world can be loaded back.
- **World info read from the files:** version, mode, difficulty, seed, in-game day, weather, spawn, player position, health and level, play time, deaths, mobs killed, distance travelled, advancements, size on disk, explored regions per dimension, data packs, game rules, and mod loaders.
- **Players by name.** Position, statistics and advancements all belong to one player: the world's owner, or the one who played the most in a world without one. In worlds several people played (LAN, a copied server world), pick another player on the world or save page; a save is compared with the world for the same player. Names come from the launcher's `usercache.json` (the official launcher's, or the Prism, Modrinth or CurseForge instance's); players it never saw show as their UUID.
- **Compare a save with the world now.** A save's page shows what it contains and, while the world is on this computer, what changed in it since: play time, which advancements were earned, regions explored per dimension, the game version, and game rules with a different value.
- **Advancement tree.** The world page can show the in-game advancement screen with the player's progress, drawn by [mcwidgets](https://github.com/KabanFriends/mcwidgets). It is off until you click **Show advancement tree**: the widget loads from `mcwidgets.kaban.sh` in a sandboxed iframe, so it needs internet access, and the world's advancement progress is sent to that site. Only vanilla advancements are shown. Worldkeeper does not ship Minecraft textures itself.
- **Plain zip files or a restic repository.** By default every save is an ordinary `.zip` file, so you can restore a world with any archive tool even without Worldkeeper. Alternatively, saves can go to a [restic](https://restic.net) repository: encrypted and deduplicated, so dozens of saves of a big world take little extra space.

See the [roadmap](ROADMAP.md) for what is planned next.

## Install

Download the file for your system from [Releases](https://github.com/dkrasiev/worldkeeper/releases):

| System | File |
|---|---|
| Windows (most PCs) | `windows_amd64.zip` |
| Windows on ARM | `windows_arm64.zip` |
| macOS (Apple Silicon and Intel) | `macOS.zip`: the app |
| macOS, command line only | `darwin_arm64.tar.gz` (Apple Silicon) / `darwin_amd64.tar.gz` (Intel) |
| Linux | `linux_amd64.tar.gz` / `linux_arm64.tar.gz` |

**Windows and Linux:** unpack the archive and run `worldkeeper`. On the first start your browser opens the UI at `http://127.0.0.1:25599`.

**macOS:** unzip `Worldkeeper_<version>_macOS.zip`, move **Worldkeeper.app** to Applications, and open it. It then shows up in Launchpad and Spotlight, and it lives in the menu bar only, with no Dock icon. On the first start your browser opens the UI at `http://127.0.0.1:25599`. The command line is inside the app: `/Applications/Worldkeeper.app/Contents/MacOS/worldkeeper list`.

**The programs are not signed** (macOS signing is tracked in [#23](https://github.com/dkrasiev/worldkeeper/issues/23)), so the system warns on the first start:

- **Windows:** SmartScreen may warn you the first time you run it. Click "More info" and then "Run anyway", or build it yourself from source.
- **macOS:** the first time, right-click (or Control-click) Worldkeeper in Applications, choose **Open**, then **Open** again. On macOS 15 Sequoia and later, open it once, then go to **System Settings → Privacy & Security**, click **Open Anyway** next to the message about Worldkeeper, and confirm. After that it opens normally. For the command-line archives, clear the quarantine flag instead: `xattr -d com.apple.quarantine ./worldkeeper`.

### Tray icon

While running, Worldkeeper sits in the system tray (Windows), the menu bar (macOS), or the status area (Linux). Its menu:

- shows when the last backup happened, or which world failed. The icon gets a red dot while a backup is failing;
- **Open Worldkeeper** opens the web UI;
- **Back up now** backs up every world that changed since its last save;
- **Back up when a world is closed** toggles the automatic backup;
- **Start at login** toggles [starting with your computer](#start-with-your-computer);
- **Download version …** appears when a newer release is out and opens its page;
- **Quit Worldkeeper** stops it.

The menu is in English or Russian. It follows the system language until you pick a language in the web UI's top bar; from then on it follows that pick, switching right away, and keeps it after a restart. With the tray available, the browser opens on its own only on the very first start. Pass `-no-tray` to run without the icon, for example on a server. On Linux the icon needs a desktop with StatusNotifier support (KDE, or GNOME with the AppIndicator extension), and Worldkeeper runs without it when there is no D-Bus session.

The log is written next to the config, in `worldkeeper.log`. The Activity feed (the last 100 events) is kept there too, in `events.json`, so failures from before a restart stay visible.

### Notifications

When a world's backup fails, Worldkeeper shows a desktop notification: a toast on Windows, a Notification Center banner on macOS, or a notification from your desktop's notification service on Linux. While the backup keeps failing and is retried, it does not notify again; it shows one more notification when the world is backed up again. Notifications use the same language as the tray menu. Turn them off under **Settings → Show a notification when a backup fails and when it works again**. On macOS they are shown through AppleScript, so System Settings → Notifications may list them under Script Editor.

### Updates

Worldkeeper checks for a new release on start and then once a day. When one is out, the web UI shows a banner and the tray menu gets a **Download version …** item. It only tells you; to update, download the new archive and replace the program with it (on macOS, Worldkeeper.app in Applications). The check is a single request to the GitHub API for the latest release, and nothing about you or your worlds is sent. Turn it off under **Settings → Check for new versions once a day**; **Check now** next to it still works. Development builds never check.

**Test builds.** On Windows x64, **Settings → Offer updates → Test builds** also offers the newest build of the `develop` branch, as soon as a change is merged there. These builds are for trying out what comes next and can have bugs, backups included. **Download** then fetches the build itself. Switch back to **Releases** any time; you are offered the next release once it is out. Under **Branch** you can follow a pull request's branch instead of `develop`, while the pull request is open; once it is merged or closed you get `develop`'s builds again. **Test builds to download** lists the last five builds of `develop` and the latest build of each open pull request, each with its own download link.

### Start with your computer

Turn on **Settings → Start Worldkeeper when you log in**, or **Start at login** in the tray menu. Worldkeeper then starts at login in the tray, without opening the browser. It registers itself the usual way for your OS:

- **Windows:** a `Worldkeeper` value under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. It is listed in Task Manager → Startup apps, and turning it off there turns it off in Worldkeeper too. A shortcut to Worldkeeper you put in `shell:startup` earlier counts as on, and is replaced by the registry value when you change the setting.
- **macOS:** a LaunchAgent, `~/Library/LaunchAgents/io.github.dkrasiev.worldkeeper.plist`, that starts the program inside Worldkeeper.app (or the command-line one you run). macOS may list it under System Settings → General → Login Items → **Allow in the Background**.
- **Linux:** an autostart entry, `~/.config/autostart/io.github.dkrasiev.worldkeeper.desktop`, which GNOME, KDE and most other desktops start at login.

If you move the program, start it once from its new place and the entry follows it. On macOS and Linux, if you set up autostart by hand before (Worldkeeper under Login Items → **Open at Login**, a systemd user service, your own autostart entry), remove that one so Worldkeeper does not start twice. To uninstall, turn the setting off before deleting the program.

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

## Which automatic saves are kept

After each save, Worldkeeper deletes the automatic saves of that world that none of these counts in **Settings → Automatic saves to keep per world** keeps:

- **Newest:** the newest N saves (20 by default).
- **Daily:** the last save of each of the N most recent days that have saves.
- **Weekly:** the same for weeks, Monday to Sunday.
- **Monthly:** the same for calendar months.

A save stays if any count keeps it, and 0 turns a count off. For example, newest 5, daily 7, weekly 4, and monthly 6 keep the last five saves and the last save of each of the last seven days, four weeks, and six months you played in. Days without saves are skipped, so a week-long break does not use up the daily saves. Days, weeks, and months follow your computer's clock. These are the rules of restic's `forget --keep-last/--keep-daily/--keep-weekly/--keep-monthly`, and with restic storage Worldkeeper runs exactly that, limited to the world's automatic saves.

Named saves and the save made before a world is deleted are never deleted automatically. The safety save made before a restore counts as an automatic save.

## Limitations

- Java Edition only. Bedrock is not supported.
- Switching between zip and restic does not migrate existing saves. Each storage keeps its own saves.
- With zip storage, every save contains the whole world. Large worlds with many saves take a lot of space, so use restic storage or keep fewer automatic saves in Settings.

## Development

You need Go 1.26+, Node 24+, and pnpm. On macOS the menu bar icon needs cgo (Xcode command line tools). Without cgo, macOS builds run without the icon.

```bash
make test     # go vet, go test, TypeScript typecheck, UI tests
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
| `internal/icon` | The grass-block icon, drawn in code, for the tray and the macOS app |
| `internal/shell` | Desktop integration: starting at login (Run key, LaunchAgent, XDG autostart) |
| `tools/macapp` | Packs the macOS release build into `Worldkeeper.app` and zips it |
| `web/` | React + Vite UI; translations in `web/src/i18n/` |

**Dev builds.** Every pull request and every push to `develop` gets a Windows x64 test build (`worldkeeper.exe`, versioned after the next minor release and the branch, like `0.3.0-develop.202610111530.abc1234` or `0.3.0-feat-autostart.202610111530.abc1234`), made once per commit. Test builds are published as pre-releases anyone can download: `dev` is the newest build of `develop`, `dev-<version>` keeps its last five builds, and `dev-<branch>` is the latest build of an open pull request (deleted when the pull request closes). Builds of forks and Dependabot pull requests stay in the *Dev build* run in the [Actions tab](https://github.com/dkrasiev/worldkeeper/actions/workflows/dev-build.yml), under *Artifacts*: you need to be signed in to GitHub, and they are kept for 14 days. Dev builds are for testing and are not releases.

**macOS app.** Pull requests that change the packaging (`.goreleaser.yaml`, `tools/macapp`, the icon) also build `Worldkeeper.app` on a Mac, check it and start it once, in the [*macOS app* workflow](https://github.com/dkrasiev/worldkeeper/actions/workflows/macos-app.yml). The zip is under *Artifacts* of that run, kept for 14 days.

### Pull requests and commits

Pull requests are squash-merged: each one becomes a single commit on `develop`, titled after the pull request. So the pull request title follows [Conventional Commits](https://www.conventionalcommits.org/): `type: what changed`, e.g. `feat: show progress for long backups` or `fix: no console window for restic on Windows`. Commits inside a pull request can be anything.

| Type | For | In release notes |
|---|---|---|
| `feat` | new features | New features |
| `fix`, `perf` | bug fixes, speedups | Bug fixes |
| `build` | packaging, installers | Other changes |
| `docs`, `test`, `ci`, `chore`, `refactor` | everything users do not see | left out |

### Releasing

Pushing a `v*` tag builds the release: GoReleaser makes the archives, `Worldkeeper.app` for macOS (a universal binary, bundled and zipped by `tools/macapp`), and the GitHub release, and opens the winget pull request if the `WINGET_TOKEN` secret is set. The tag can be on any commit.

**Release notes are generated, not written.** GoReleaser lists the pull requests merged since the previous `v*` tag, grouped by their Conventional Commits type (see the table above), between a fixed header and an install footer, both defined in `.goreleaser.yaml`. So a pull request title is the line users read in the release notes.

**Branches.** Pull requests go into `develop`, the default branch. `main` holds the latest release. A release fast-forwards `main` to `develop` and is never merged, so both branches keep the same commits (pull requests are squash-merged, and merging `develop` into `main` would squash a whole release into one commit).

**Regular release** of `develop`, once its CI is green:

```bash
git switch main && git pull
git merge --ff-only origin/develop
git push origin main
git tag -a v0.3.0 -m "v0.3.0"
git push origin v0.3.0
```

**Hotfix** for a past release, when `develop` already has unreleased work. Merge the fix into `develop` first, then:

```bash
git switch -c release/v0.2 v0.2.0   # or reuse release/v0.2 from an earlier hotfix
git cherry-pick <fix-commit>
git push -u origin release/v0.2     # CI runs on release/* branches
git tag -a v0.2.1 -m "v0.2.1"
git push origin v0.2.1
```

Keep `release/v0.2` for later hotfixes (`v0.2.2`, …). If `develop` has nothing unreleased, make a regular release instead.

## License

MIT
