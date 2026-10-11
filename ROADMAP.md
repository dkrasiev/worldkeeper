# Roadmap

Worldkeeper is a personal project shared as-is, so this is a plan, not a promise. Each item is a GitHub issue grouped into [milestones](https://github.com/dkrasiev/worldkeeper/milestones). Ideas and votes (👍 on an issue) are welcome.

## [v0.2](https://github.com/dkrasiev/worldkeeper/milestone/1): works reliably

Make what already shipped trustworthy before adding more.

- [#6](https://github.com/dkrasiev/worldkeeper/issues/6) Verify v0.1 on Windows: tray, auto-backup, restic, CLI
- [#7](https://github.com/dkrasiev/worldkeeper/issues/7) Verify world info on real 1.16–1.21 saves
- [#8](https://github.com/dkrasiev/worldkeeper/issues/8) Keep the activity log across restarts
- [#9](https://github.com/dkrasiev/worldkeeper/issues/9) Show progress for long backups and restores
- [#10](https://github.com/dkrasiev/worldkeeper/issues/10) "Load as a new world" keeps the original world name
- [#11](https://github.com/dkrasiev/worldkeeper/issues/11) Open-source housekeeping
- [#55](https://github.com/dkrasiev/worldkeeper/issues/55) Rename and delete worlds

## [v0.3](https://github.com/dkrasiev/worldkeeper/milestone/2): convenience

- [#12](https://github.com/dkrasiev/worldkeeper/issues/12) Consistent player data, player switcher and names
- [#13](https://github.com/dkrasiev/worldkeeper/issues/13) Start with the system from settings and the tray
- [#14](https://github.com/dkrasiev/worldkeeper/issues/14) Desktop notification when a backup fails
- [#15](https://github.com/dkrasiev/worldkeeper/issues/15) Daily / weekly / monthly retention (GFS)
- [#16](https://github.com/dkrasiev/worldkeeper/issues/16) Compare a save with the current world
- [#17](https://github.com/dkrasiev/worldkeeper/issues/17) Tray menu language follows the UI language
- [#39](https://github.com/dkrasiev/worldkeeper/issues/39) Windows: app icon, version info and Start menu entry
- [#40](https://github.com/dkrasiev/worldkeeper/issues/40) macOS: `Worldkeeper.app` bundle (Launchpad, Spotlight, menu bar only)
- [#59](https://github.com/dkrasiev/worldkeeper/issues/59) Notify about new releases

## [v0.4](https://github.com/dkrasiev/worldkeeper/milestone/3): storage, sync and distribution

- [#18](https://github.com/dkrasiev/worldkeeper/issues/18) Restic cloud backends (S3, B2) with credentials in the keychain
- [#19](https://github.com/dkrasiev/worldkeeper/issues/19) Scheduled restic prune and check
- [#20](https://github.com/dkrasiev/worldkeeper/issues/20) Move saves between zip and restic storage
- [#21](https://github.com/dkrasiev/worldkeeper/issues/21) Periodic restore test
- [#41](https://github.com/dkrasiev/worldkeeper/issues/41) Linux: desktop entry, icons and `.deb`/`.rpm` packages
- [#80](https://github.com/dkrasiev/worldkeeper/issues/80) Recognize a world across computers and tell which copy is newer
- [#81](https://github.com/dkrasiev/worldkeeper/issues/81) Sync chosen worlds between computers automatically
- [#82](https://github.com/dkrasiev/worldkeeper/issues/82) Warn when a world is open on another computer, or needs a newer game

## [Later](https://github.com/dkrasiev/worldkeeper/milestone/4)

- [#24](https://github.com/dkrasiev/worldkeeper/issues/24) More launchers: MultiMC, ATLauncher, GDLauncher
- [#25](https://github.com/dkrasiev/worldkeeper/issues/25) Server worlds via RCON
- [#26](https://github.com/dkrasiev/worldkeeper/issues/26) Bedrock Edition
- [#27](https://github.com/dkrasiev/worldkeeper/issues/27) Mod and datapack advancements in the tree
- [#28](https://github.com/dkrasiev/worldkeeper/issues/28) Other games via manifests
- [#35](https://github.com/dkrasiev/worldkeeper/issues/35) Companion Minecraft mod: save from the pause menu, consistent backups while playing, in-game status
- [#44](https://github.com/dkrasiev/worldkeeper/issues/44) Publish to winget

## Not planned

- **Bundling restic or writing a custom deduplicating format.** Saves stay plain zip files or a standard restic repository, so they can always be restored without Worldkeeper.
- **A hosted service, accounts, or telemetry.** Everything stays on your machine and your storage.
- **Code signing and notarization.** A Windows code signing certificate and Apple's developer program cost money every year. The programs stay unsigned; the README shows how to start them the first time.

## Branches and releases

Pull requests target `develop`, the default branch, and every pull request runs CI on Windows, macOS and Linux. A release fast-forwards `main` to `develop` and tags it (`v*`); GoReleaser builds and publishes it.
