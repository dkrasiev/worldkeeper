// Typed client for the worldkeeper local API. Types mirror the Go JSON.

export type GameMode = "survival" | "creative" | "adventure" | "spectator" | "";
export type SnapshotKind = "auto" | "manual" | "pre-restore" | "pre-delete" | "pre-move";
export type RestoreMode = "replace" | "copy";

/** A saves folder worlds are found in, and can be moved or restored into. */
export interface Location {
  source: string;
  label: string; // e.g. "Prism Launcher: Fabric 1.21"
  saves: string;
}

export interface Snapshot {
  id: string;
  kind: SnapshotKind;
  label?: string;
  note?: string;
  createdAt: string;
  sizeBytes: number;
  gameVersion?: string;
  lastPlayed: string;
}

export interface WorldRef {
  id: string;
  name: string;
  folder: string;
  source: string;
  sourceLabel: string;
  path: string;
}

export interface SnapshotIndex {
  world: WorldRef;
  snapshots: Snapshot[] | null;
}

export interface WorldView {
  id: string;
  folder: string;
  path: string;
  source: string;
  sourceLabel: string;
  savesDir: string;
  name: string;
  gameVersion: string;
  dataVersion: number;
  gameMode: GameMode;
  hardcore: boolean;
  lastPlayed: string;
  hasIcon: boolean;
  modded: boolean;
  inUse: boolean;
  error?: string;
  snapshots: number;
  lastBackup?: Snapshot;
  changed: boolean;
}

export interface Overview {
  worlds: WorldView[];
  archived: SnapshotIndex[];
  storageError?: string;
}

export interface PlayerRef {
  uuid: string;
  /** From the launcher's usercache.json; missing for players it never saw. */
  name?: string;
  owner?: boolean;
  playTimeSeconds: number;
}

/** Everything read from a world's files, live or from a save. */
export interface WorldData {
  name: string;
  gameVersion: string;
  dataVersion: number;
  gameMode: GameMode;
  hardcore: boolean;
  lastPlayed: string;
  hasIcon: boolean;
  modded: boolean;
  snapshotVersion: boolean;
  difficulty: string;
  difficultyLocked: boolean;
  cheats: boolean;
  seed?: string;
  day: number;
  weather: "clear" | "rain" | "thunder";
  spawn?: number[];
  brands: string[];
  dataPacks: { enabled: string[]; disabled: string[] };
  gameRules: Record<string, string>;
  /** Everyone with data in the world, the main player (owner, else most play time) first. */
  players: PlayerRef[];
  /** Whom player, stats and advancements describe. */
  playerUuid?: string;
  player?: { dimension: string; pos?: number[]; health: number; food: number; xpLevel: number };
  stats?: {
    playTimeSeconds: number;
    deaths: number;
    mobKills: number;
    distanceMeters: number;
    jumps: number;
  };
  advancements: number;
  sizeBytes: number;
  dimensions: { id: string; regionFiles: number }[];
}

/** A world on this computer: its files plus where it lives. */
export interface WorldInfo extends WorldData {
  id: string;
  folder: string;
  path: string;
  sourceLabel: string;
  savesDir: string; // the saves folder the world is in
  inUse: boolean;
  /** It can be started from Worldkeeper: a world in a Prism Launcher instance. */
  playable: boolean;
}

export type Engine = "zip" | "restic";

export interface Settings {
  engine: Engine;
  storageDir: string;
  restic: { repo: string; binary: string; passwordFile: string };
  resticPasswordSet?: boolean;
  resticPassword?: string; // write-only
  extraSavesDirs: string[] | null;
  /** Rotation keeps an automatic save if any of these counts keeps it; 0 turns a count off. */
  keepAuto: number; // the newest N
  keepDaily: number; // the newest save of each of the last N days with saves
  keepWeekly: number;
  keepMonthly: number;
  autoBackup: boolean;
  pollSeconds: number;
  checkUpdates: boolean;
  /** "dev" also offers test builds of the develop branch. */
  updateChannel: "stable" | "dev";
  /** Branch whose test builds the "dev" channel offers: "develop" or a pull request branch. */
  updateBranch: string;
  /** Desktop notification when a world's backups fail and recover. */
  notifications: boolean;
  /** Read from the OS; absent where autostart is unavailable. */
  startWithSystem?: boolean;
}

/** New-release check (GET /api/update). */
export interface UpdateStatus {
  current: string;
  /** False for local builds ("dev"), which are never checked. */
  versioned: boolean;
  /** Test builds are made for this platform (Windows x64 only). */
  devBuilds: boolean;
  latest?: {
    version: string;
    url: string; // release page
    download?: string; // the archive itself; set for test builds
    dev?: boolean; // a test build
    branch?: string; // the branch a test build was made from
  };
  available: boolean;
  checkedAt?: string; // last successful check
  error?: string; // last check failed
}

export interface ResticStatus {
  version?: string;
  state: "ok" | "not_installed" | "missing" | "wrong_password" | "no_password" | "error";
  message?: string;
}

export interface ActivityEvent {
  time: string;
  kind: "backup" | "restore" | "error" | "world";
  worldId: string;
  world: string;
  message: string; // English fallback
  code?:
    | "backup_saved"
    | "backup_failed"
    | "restore_done"
    | "restore_failed"
    | "world_renamed"
    | "world_deleted"
    | "delete_failed"
    | "world_moved"
    | "move_failed";
  params?: Record<string, string>;
}

/** A backup or restore running in the background (GET /api/jobs). */
export interface Job {
  id: string;
  kind: "backup" | "restore" | "delete" | "move";
  worldId: string;
  world: string;
  auto?: boolean; // started by the automatic backup
  state: "running" | "done" | "failed";
  /** A restore over an existing world saves its current state first ("safety"). */
  phase: "backup" | "safety" | "restore" | "delete" | "move";
  done: number; // bytes
  total: number; // bytes; 0 while unknown
  startedAt: string;
  finishedAt?: string;
  snapshot?: Snapshot; // backup result
  path?: string; // restore destination, the deleted folder, or where a world moved to
  newWorldId?: string; // a moved world's id, which follows its location
  error?: { error: string; message: string };
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    /** Translation key for the UI (see err.* messages). */
    public key?: string,
    public params?: Record<string, string>,
  ) {
    super(message);
  }
}

const TOKEN_KEY = "worldkeeper.token";

// The token arrives once in the URL opened by the app, then lives in storage.
function initToken(): string {
  const params = new URLSearchParams(location.search);
  const fromUrl = params.get("token");
  if (fromUrl) {
    try {
      localStorage.setItem(TOKEN_KEY, fromUrl);
    } catch {
      /* storage blocked: keep it in memory only */
    }
    params.delete("token");
    const q = params.toString();
    history.replaceState(null, "", location.pathname + (q ? "?" + q : "") + location.hash);
    return fromUrl;
  }
  try {
    return localStorage.getItem(TOKEN_KEY) ?? "";
  } catch {
    return "";
  }
}

export const token = initToken();

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: {
      "X-Worldkeeper-Token": token,
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? "unknown", data.message ?? res.statusText, data.key, data.params);
  }
  return data as T;
}

const w = (id: string) => `/api/worlds/${encodeURIComponent(id)}`;
// Readers take an optional player UUID; without one the server shows the main player.
const p = (player?: string) => (player ? `?player=${encodeURIComponent(player)}` : "");

export const api = {
  overview: () => request<Overview>("GET", "/api/overview"),
  events: () => request<ActivityEvent[]>("GET", "/api/events"),
  about: () => request<{ version: string; os: string; arch: string }>("GET", "/api/about"),
  update: () => request<UpdateStatus>("GET", "/api/update"),
  /** Asks GitHub for the latest release now. */
  checkUpdate: () => request<UpdateStatus>("POST", "/api/update/check"),
  /** Published test builds: develop first, then open pull requests. */
  testBuilds: () => request<NonNullable<UpdateStatus["latest"]>[]>("GET", "/api/update/builds"),
  world: (id: string, player?: string) => request<WorldInfo>("GET", w(id) + p(player)),
  advancements: (id: string, player?: string) =>
    request<Record<string, unknown>>("GET", `${w(id)}/advancements${p(player)}`),
  snapshotInfo: (id: string, snap: string, player?: string) =>
    request<WorldData>("GET", `${w(id)}/snapshots/${encodeURIComponent(snap)}/info${p(player)}`),
  snapshotAdvancements: (id: string, snap: string, player?: string) =>
    request<Record<string, unknown>>(
      "GET",
      `${w(id)}/snapshots/${encodeURIComponent(snap)}/advancements${p(player)}`,
    ),
  snapshots: (id: string) => request<SnapshotIndex>("GET", `${w(id)}/snapshots`),
  jobs: () => request<Job[]>("GET", "/api/jobs"),
  /** Changes the name shown in the game; the folder keeps its name. */
  rename: (id: string, name: string) => request<{ name: string }>("PATCH", w(id), { name }),
  /** Starts deleting a world (saved first unless already backed up); follow it with jobs(). */
  deleteWorld: (id: string) => request<Job>("DELETE", w(id)),
  /** Starts moving a world into another saves folder; follow it with jobs(). */
  moveWorld: (id: string, savesDir: string) => request<Job>("POST", `${w(id)}/move`, { savesDir }),
  /** Shows the world's folder in the file manager. */
  openFolder: (id: string) => request<{ ok: boolean }>("POST", `${w(id)}/open`),
  /** Starts the world in Prism Launcher (see WorldInfo.playable). */
  play: (id: string) => request<{ ok: boolean }>("POST", `${w(id)}/play`),
  /** Saves folders a world can be moved or restored into. */
  locations: () => request<Location[]>("GET", "/api/locations"),
  /** Starts a backup; follow it with jobs(). */
  save: (id: string, label: string, note: string, force = false) =>
    request<Job>("POST", `${w(id)}/snapshots`, { label, note, force }),
  /** Starts a restore; follow it with jobs(). savesDir picks the folder for a copy or a world without a local folder. */
  restore: (id: string, snap: string, mode: RestoreMode, savesDir?: string) =>
    request<Job>("POST", `${w(id)}/snapshots/${encodeURIComponent(snap)}/restore`, { mode, savesDir }),
  deleteSnapshot: (id: string, snap: string) =>
    request<{ ok: boolean }>("DELETE", `${w(id)}/snapshots/${encodeURIComponent(snap)}`),
  settings: () => request<Settings>("GET", "/api/config"),
  saveSettings: (s: Settings) => request<Settings>("PUT", "/api/config", s),
  /** Tells the server the UI language for the tray menu; "" follows the OS language. */
  setLanguage: (language: string) =>
    request<{ language: string; locale: string }>("PUT", "/api/language", { language }),
  resticCheck: () => request<ResticStatus>("POST", "/api/restic/check"),
  resticInit: () => request<ResticStatus>("POST", "/api/restic/init"),
  iconUrl: (id: string) => `${w(id)}/icon?token=${encodeURIComponent(token)}`,
};
