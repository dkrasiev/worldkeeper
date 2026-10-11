import type { WorldData } from "./api";

// Comparing a save with the live world happens here, in the browser: the save
// page already has both payloads. Kept free of React and runtime imports so
// `node --test` can run it directly (see web/test/).

/** Advancement progress as the API returns it: id -> true once done. */
export type Progress = Record<string, unknown>;

export interface Change<T> {
  from: T; // in the save
  to: T; // in the world now
}

/** What differs between a save and the world as it is now. */
export interface WorldDiff {
  /** The save was played later than the world now, e.g. an older save was loaded since. */
  saveIsNewer: boolean;
  version?: Change<string>;
  playTimeSeconds?: Change<number>;
  advancements?: Change<number> & { gained: string[]; lost: string[] };
  /** Region files per dimension, only where the count changed. */
  regions: (Change<number> & { id: string })[];
  /** Rules set in both with a different value; rules added by a new version are left out. */
  gameRules: (Change<string> & { rule: string })[];
}

const done = (p: Progress | undefined) =>
  new Set(
    Object.entries(p ?? {})
      .filter(([, v]) => v === true)
      .map(([id]) => id),
  );

// Go writes a zero time for "unknown"; anything before 2000 is not a real date.
const time = (iso: string) => {
  const ms = Date.parse(iso);
  return ms > Date.UTC(2000, 0) ? ms : undefined;
};

/**
 * compareWorlds lists what changed from a save to the world now. The
 * progress maps name the advancements gained and lost; without them only
 * the counts are compared. Play time and advancements are a player's, so
 * they are compared only when both sides describe the same player (the
 * live world may not have the save's player, or not yet be reloaded for it).
 */
export function compareWorlds(save: WorldData, now: WorldData, saveAdv?: Progress, nowAdv?: Progress): WorldDiff {
  const saved = time(save.lastPlayed);
  const played = time(now.lastPlayed);
  const samePlayer = save.playerUuid === now.playerUuid;
  // Play time only grows, so it tells the order even when the dates do not.
  const longer =
    samePlayer && !!save.stats && !!now.stats && save.stats.playTimeSeconds > now.stats.playTimeSeconds;
  const diff: WorldDiff = {
    saveIsNewer: (saved !== undefined && played !== undefined && saved > played) || longer,
    regions: [],
    gameRules: [],
  };

  if (save.gameVersion && now.gameVersion && save.gameVersion !== now.gameVersion) {
    diff.version = { from: save.gameVersion, to: now.gameVersion };
  }

  if (samePlayer && (save.stats || now.stats)) {
    const from = save.stats?.playTimeSeconds ?? 0;
    const to = now.stats?.playTimeSeconds ?? 0;
    if (from !== to) diff.playTimeSeconds = { from, to };
  }

  let gained: string[] = [];
  let lost: string[] = [];
  if (samePlayer && saveAdv && nowAdv) {
    const a = done(saveAdv);
    const b = done(nowAdv);
    gained = [...b].filter((id) => !a.has(id)).sort();
    lost = [...a].filter((id) => !b.has(id)).sort();
  }
  if (samePlayer && (save.advancements !== now.advancements || gained.length > 0 || lost.length > 0)) {
    diff.advancements = { from: save.advancements, to: now.advancements, gained, lost };
  }

  const before = new Map(save.dimensions.map((d) => [d.id, d.regionFiles]));
  const ids = [...now.dimensions.map((d) => d.id), ...save.dimensions.map((d) => d.id)];
  const after = new Map(now.dimensions.map((d) => [d.id, d.regionFiles]));
  for (const id of new Set(ids)) {
    const from = before.get(id) ?? 0;
    const to = after.get(id) ?? 0;
    if (from !== to) diff.regions.push({ id, from, to });
  }

  for (const rule of Object.keys(save.gameRules).sort()) {
    const from = save.gameRules[rule];
    const to = now.gameRules[rule];
    if (to !== undefined && from !== to) diff.gameRules.push({ rule, from, to });
  }

  return diff;
}

/** True when nothing compared differs. */
export function unchanged(d: WorldDiff): boolean {
  return !d.version && !d.playTimeSeconds && !d.advancements && d.regions.length === 0 && d.gameRules.length === 0;
}
