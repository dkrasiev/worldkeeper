// Typed client for the worldkeeper local API. Types mirror the Go JSON.

export type GameMode = "survival" | "creative" | "adventure" | "spectator" | "";
export type SnapshotKind = "auto" | "manual" | "pre-restore";
export type RestoreMode = "replace" | "copy";

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
  inUse: boolean;
}

export type Engine = "zip" | "restic";

export interface Settings {
  engine: Engine;
  storageDir: string;
  restic: { repo: string; binary: string; passwordFile: string };
  resticPasswordSet?: boolean;
  resticPassword?: string; // write-only
  extraSavesDirs: string[] | null;
  keepAuto: number;
  autoBackup: boolean;
  pollSeconds: number;
}

export interface ResticStatus {
  version?: string;
  state: "ok" | "not_installed" | "missing" | "wrong_password" | "no_password" | "error";
  message?: string;
}

export interface ActivityEvent {
  time: string;
  kind: "backup" | "restore" | "error";
  worldId: string;
  world: string;
  message: string; // English fallback
  code?: "backup_saved" | "backup_failed" | "restore_done" | "restore_failed";
  params?: Record<string, string>;
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

export const api = {
  overview: () => request<Overview>("GET", "/api/overview"),
  events: () => request<ActivityEvent[]>("GET", "/api/events"),
  about: () => request<{ version: string; os: string; arch: string }>("GET", "/api/about"),
  world: (id: string) => request<WorldInfo>("GET", w(id)),
  advancements: (id: string) => request<Record<string, unknown>>("GET", `${w(id)}/advancements`),
  snapshotInfo: (id: string, snap: string) =>
    request<WorldData>("GET", `${w(id)}/snapshots/${encodeURIComponent(snap)}/info`),
  snapshotAdvancements: (id: string, snap: string) =>
    request<Record<string, unknown>>("GET", `${w(id)}/snapshots/${encodeURIComponent(snap)}/advancements`),
  snapshots: (id: string) => request<SnapshotIndex>("GET", `${w(id)}/snapshots`),
  save: (id: string, label: string, note: string, force = false) =>
    request<Snapshot>("POST", `${w(id)}/snapshots`, { label, note, force }),
  restore: (id: string, snap: string, mode: RestoreMode) =>
    request<{ path: string }>("POST", `${w(id)}/snapshots/${encodeURIComponent(snap)}/restore`, { mode }),
  deleteSnapshot: (id: string, snap: string) =>
    request<{ ok: boolean }>("DELETE", `${w(id)}/snapshots/${encodeURIComponent(snap)}`),
  settings: () => request<Settings>("GET", "/api/config"),
  saveSettings: (s: Settings) => request<Settings>("PUT", "/api/config", s),
  resticCheck: () => request<ResticStatus>("POST", "/api/restic/check"),
  resticInit: () => request<ResticStatus>("POST", "/api/restic/init"),
  iconUrl: (id: string) => `${w(id)}/icon?token=${encodeURIComponent(token)}`,
};
