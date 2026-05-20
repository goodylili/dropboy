import type { FileNode, Machine } from "./types";

const TOKEN_KEY = "dropboy.token";
const CSRF_HEADER = "X-Dropboy-CSRF";

function apiBase(): string {
  if (typeof window === "undefined") return "";
  return process.env.NEXT_PUBLIC_API_BASE ?? "";
}

// captureTokenFromURL pulls ?token=... from the current URL on first load,
// stashes it in localStorage, and strips it from the address bar.
export function captureTokenFromURL(): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  const t = url.searchParams.get("token");
  if (t) {
    window.localStorage.setItem(TOKEN_KEY, t);
    url.searchParams.delete("token");
    window.history.replaceState({}, "", url.toString());
  }
}

function token(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_KEY) ?? "";
}

export function hasToken(): boolean {
  return token() !== "";
}

async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  const tok = token();
  if (tok) {
    headers.set("Authorization", `Bearer ${tok}`);
    const method = (init.method ?? "GET").toUpperCase();
    if (method !== "GET" && method !== "HEAD") {
      headers.set(CSRF_HEADER, tok);
      if (!headers.has("Content-Type") && init.body) {
        headers.set("Content-Type", "application/json");
      }
    }
  }
  const res = await fetch(apiBase() + path, {
    ...init,
    headers,
    credentials: "include",
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get("content-type") ?? "";
  if (ct.includes("application/json")) return res.json() as Promise<T>;
  return res.text() as unknown as T;
}

// ---- typed wrappers ----

export interface Folder {
  path: string;
  exclude?: string[];
}

export interface Status {
  running: boolean;
  locked: boolean;
  paused: boolean;
  queueUploads: number;
  queueDownloads: number;
  bytesUp: number;
  bytesDown: number;
  conflicts: number;
  lastSyncAt: string;
  bucket: string;
  region: string;
  machineId: string;
}

export interface Conflict {
  id: string;
  path: string;
  machine: string;
  detected: string;
  local: string;
  remote: string;
}

export const api = {
  machines: () => request<Machine[]>("/api/v1/machines"),
  tree: (machine: string, path = "/") =>
    request<FileNode>(`/api/v1/tree?machine=${encodeURIComponent(machine)}&path=${encodeURIComponent(path)}`),
  status: () => request<Status>("/api/v1/status"),
  folders: () => request<Folder[]>("/api/v1/folders"),
  addFolder: (p: Folder) =>
    request<Folder[]>("/api/v1/folders", { method: "POST", body: JSON.stringify(p) }),
  removeFolder: (path: string) =>
    request<Folder[]>(`/api/v1/folders?path=${encodeURIComponent(path)}`, { method: "DELETE" }),
  conflicts: () => request<Conflict[]>("/api/v1/conflicts"),
  resolveConflict: (id: string, resolution: "local" | "remote" | "both") =>
    request<{ status: string }>(`/api/v1/conflicts/resolve`, {
      method: "POST",
      body: JSON.stringify({ id, resolution }),
    }),
  restore: (p: { machine: string; path?: string; into?: string }) =>
    request<{ status: string }>("/api/v1/restore", { method: "POST", body: JSON.stringify(p) }),
  pause: () => request<{ paused: boolean }>("/api/v1/pause", { method: "POST" }),
  resume: () => request<{ paused: boolean }>("/api/v1/resume", { method: "POST" }),
  sync: () => request<{ status: string }>("/api/v1/sync", { method: "POST" }),
  unlock: (passphrase: string, remember: boolean) =>
    request<{ unlocked: boolean; remembered: boolean }>("/api/v1/unlock", {
      method: "POST",
      body: JSON.stringify({ passphrase, remember }),
    }),
  forgetPassphrase: () =>
    request<{ forgotten: boolean }>("/api/v1/forget-passphrase", { method: "POST" }),
  fileURL: (machine: string, path: string) => {
    const base = apiBase();
    const u = new URL(
      `${base || window.location.origin}/api/v1/file`,
    );
    u.searchParams.set("machine", machine);
    u.searchParams.set("path", path);
    const tok = token();
    if (tok) u.searchParams.set("token", tok);
    return u.toString();
  },
};

// subscribeEvents opens the SSE stream and invokes the callback for each
// event. Returns a cleanup function that closes the stream.
export function subscribeEvents(
  onEvent: (e: { type: string; time: string; payload?: unknown }) => void,
): () => void {
  if (typeof window === "undefined") return () => {};
  const tok = token();
  const url = `${apiBase()}/api/v1/events${tok ? `?token=${encodeURIComponent(tok)}` : ""}`;
  const es = new EventSource(url, { withCredentials: true });
  const handler = (ev: MessageEvent) => {
    try {
      onEvent(JSON.parse(ev.data));
    } catch {
      // ignore malformed
    }
  };
  // The Go server uses `event: <type>` framing, so listen on those types
  // we publish today plus the default "message" channel as a fallback.
  ["status", "sync.kick", "sync.paused", "sync.resumed", "message"].forEach((t) =>
    es.addEventListener(t, handler as EventListener),
  );
  return () => es.close();
}
