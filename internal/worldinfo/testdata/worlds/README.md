# Real-world fixtures

Each folder is a real Minecraft world, trimmed by `tools/trimworld`, and
`<folder>.json` is what `worldinfo` must read from it. `TestGoldenWorlds`
compares the two, so a parser change that breaks an older (or newer)
version fails CI.

Trimming keeps `level.dat`, `data/`, the player folders (`players/` in
26.x; `playerdata/`, `stats/`, `advancements/` before) and `icon.png`.
Region files become empty files with the same names, so dimensions are
still counted. Player UUIDs are replaced with fake ones in file names and
inside NBT and JSON, so a fixture never identifies a Minecraft account.

## Adding a Minecraft version

1. In your launcher (e.g. Prism), create an instance of that version and a
   new singleplayer world. Play a minute: craft a crafting table (earns an
   advancement), break a few blocks, walk around. Exit to the title screen.
2. Trim it into a fixture, named `<version>-singleplayer`:

   ```bash
   go run ./tools/trimworld "<saves folder>/<World name>" internal/worldinfo/testdata/worlds/1.20.6-singleplayer
   ```

3. Generate its expected output and review it. Check that the version, seed,
   player, statistics and advancements make sense:

   ```bash
   go test ./internal/worldinfo -run TestGoldenWorlds -update
   git diff internal/worldinfo/testdata/worlds/
   ```

4. Commit both the folder and the `.json`.

If a fixture's output changes after a code change, rerun with `-update` and
review the diff: it shows exactly what the parser now reads differently.
