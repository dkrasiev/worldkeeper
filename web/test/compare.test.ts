// Run with `pnpm test` (node --test; Node strips the types itself).
import assert from "node:assert/strict";
import { test } from "node:test";
import type { WorldData } from "../src/api.ts";
import { compareWorlds, unchanged } from "../src/compare.ts";

function world(over: Partial<WorldData> = {}): WorldData {
  return {
    name: "World",
    gameVersion: "1.21.4",
    dataVersion: 4189,
    gameMode: "survival",
    hardcore: false,
    lastPlayed: "2026-10-01T12:00:00Z",
    hasIcon: false,
    modded: false,
    snapshotVersion: false,
    difficulty: "normal",
    difficultyLocked: false,
    cheats: false,
    day: 10,
    weather: "clear",
    brands: [],
    dataPacks: { enabled: ["vanilla"], disabled: [] },
    gameRules: { keepInventory: "false", doDaylightCycle: "true" },
    stats: { playTimeSeconds: 3600, deaths: 0, mobKills: 0, distanceMeters: 0, jumps: 0 },
    advancements: 2,
    sizeBytes: 1000,
    dimensions: [{ id: "minecraft:overworld", regionFiles: 4 }],
    ...over,
  };
}

test("same world is unchanged", () => {
  const adv = { "minecraft:story/root": true };
  const d = compareWorlds(world(), world(), adv, adv);
  assert.ok(unchanged(d));
  assert.equal(d.saveIsNewer, false);
});

test("lists what changed since the save", () => {
  const save = world();
  const now = world({
    gameVersion: "1.21.5",
    lastPlayed: "2026-10-05T12:00:00Z",
    gameRules: { keepInventory: "true", doDaylightCycle: "true", newRule: "1" },
    stats: { playTimeSeconds: 7200, deaths: 1, mobKills: 0, distanceMeters: 0, jumps: 0 },
    advancements: 3,
    dimensions: [
      { id: "minecraft:overworld", regionFiles: 6 },
      { id: "minecraft:the_nether", regionFiles: 1 },
    ],
  });
  const d = compareWorlds(
    save,
    now,
    {
      "minecraft:story/root": true,
      "minecraft:story/mine_stone": true,
      "minecraft:story/smelt_iron": { criteria: { iron: true } },
    },
    { "minecraft:story/root": true, "minecraft:story/mine_stone": true, "minecraft:story/smelt_iron": true },
  );
  assert.equal(d.saveIsNewer, false);
  assert.deepEqual(d.version, { from: "1.21.4", to: "1.21.5" });
  assert.deepEqual(d.playTimeSeconds, { from: 3600, to: 7200 });
  assert.deepEqual(d.advancements, { from: 2, to: 3, gained: ["minecraft:story/smelt_iron"], lost: [] });
  assert.deepEqual(d.regions, [
    { id: "minecraft:overworld", from: 4, to: 6 },
    { id: "minecraft:the_nether", from: 0, to: 1 },
  ]);
  // A rule the new version added is not a change the player made.
  assert.deepEqual(d.gameRules, [{ rule: "keepInventory", from: "false", to: "true" }]);
});

test("save newer than the world", () => {
  const save = world({ lastPlayed: "2026-10-05T12:00:00Z", advancements: 2 });
  const now = world({ lastPlayed: "2026-10-01T12:00:00Z", advancements: 1, dimensions: [] });
  const d = compareWorlds(
    save,
    now,
    { "minecraft:story/root": true, "minecraft:story/mine_stone": true },
    { "minecraft:story/root": true },
  );
  assert.equal(d.saveIsNewer, true);
  assert.deepEqual(d.advancements, { from: 2, to: 1, gained: [], lost: ["minecraft:story/mine_stone"] });
  assert.deepEqual(d.regions, [{ id: "minecraft:overworld", from: 4, to: 0 }]);
});

test("more play time in the save means it is newer", () => {
  const save = world({ stats: { playTimeSeconds: 7200, deaths: 0, mobKills: 0, distanceMeters: 0, jumps: 0 } });
  const d = compareWorlds(save, world());
  assert.equal(d.saveIsNewer, true);
  assert.deepEqual(d.playTimeSeconds, { from: 7200, to: 3600 });
});

test("counts only without progress maps", () => {
  const d = compareWorlds(world(), world({ advancements: 5 }));
  assert.deepEqual(d.advancements, { from: 2, to: 5, gained: [], lost: [] });
});

test("unknown dates and missing stats", () => {
  const d = compareWorlds(
    world({ lastPlayed: "0001-01-01T00:00:00Z", stats: undefined }),
    world({ stats: { playTimeSeconds: 60, deaths: 0, mobKills: 0, distanceMeters: 0, jumps: 0 } }),
  );
  assert.equal(d.saveIsNewer, false);
  assert.deepEqual(d.playTimeSeconds, { from: 0, to: 60 });
});
