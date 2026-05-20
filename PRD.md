# dropboy — Product Requirements Document

## 1. Summary

`dropboy` is a cross-platform CLI tool that continuously synchronizes user-specified local folders with an AWS S3 bucket they own. It is positioned as a **self-hosted, BYO-cloud alternative to iCloud Desktop and Google Drive for Desktop**: the user keeps full ownership of their data and storage account, while dropboy provides the seamless "it just works" two-way sync experience that consumer cloud-drive products deliver.

Unlike `rclone` (manual, on-demand) or `aws s3 sync` (one-shot, one-way), dropboy runs as a background daemon that mirrors the user's filesystem layout into S3 in near real-time, applies client-side encryption before upload, and supports restoring an entire machine's tree onto a new device.

## 2. Goals & Non-Goals

### Goals
- Provide an **iCloud-Desktop-class experience** on top of a user-owned S3 bucket.
- **Two-way sync** between configured local folders and S3, with deterministic conflict resolution.
- **Per-machine namespacing** so a single bucket can back up multiple devices without collision and tree structures stay browsable in the S3 console.
- **Client-side encryption** so file contents are unreadable to anyone with bucket access (including AWS).
- A **single static Go binary** the user installs once and forgets.
- **Multi-machine restore**: bring up a new laptop, point dropboy at the bucket, recover the tree.

### Non-Goals (v1)
- Public/hosted web UI (a **localhost-only** UI is in scope — see §5.8).
- Real-time collaborative editing or sharing links.
- Non-S3 backends (GCS, Azure Blob, B2). Pluggable backend interface kept in mind, not implemented.
- Mobile clients.
- Selective sync UI (config-file only in v1).

## 3. Target User & Use Cases

**Primary user**: technical individuals (developers, sysadmins, security-minded users) who:
- Want iCloud/Drive ergonomics but distrust or refuse vendor lock-in.
- Already have or want an AWS account and prefer paying S3 storage rates directly.
- Use multiple machines (laptop + desktop, work + personal) and want the same folders mirrored.

**Use cases**:
1. **Live backup**: `~/Documents`, `~/Projects`, `~/Desktop` continuously mirrored to S3.
2. **Cross-machine sync**: edit a file on the laptop, see it appear on the desktop within minutes.
3. **Disaster recovery**: laptop is stolen; new machine pulls the full tree back via `dropboy restore`.
4. **Encrypted off-site archive**: a folder of sensitive material is mirrored to S3 with no plaintext ever leaving the device.

## 4. User Experience

### Install & first-run
```
brew install dropboy            # or: go install github.com/goodylili/dropboy@latest
dropboy init                    # interactive: bucket, region, AWS profile, encryption passphrase
dropboy add ~/Documents
dropboy add ~/Projects --exclude 'node_modules/**' --exclude '*.log'
dropboy start                   # launches daemon, registers with launchd/systemd
```

### Day-to-day
The user does nothing. Files saved into watched folders appear in S3 within seconds. Files changed on another machine running dropboy against the same bucket appear locally within the daemon's poll interval.

### CLI surface (v1)
| Command | Purpose |
|---|---|
| `dropboy init` | Configure bucket, region, credentials, encryption key, machine ID. |
| `dropboy add <path> [--exclude ...]` | Register a folder to watch. |
| `dropboy remove <path>` | Stop watching a folder (does not delete from S3). |
| `dropboy list` | List watched folders and their status. |
| `dropboy status` | Show daemon health, queue depth, last sync time, conflicts. |
| `dropboy start` / `stop` / `restart` | Daemon lifecycle (delegates to launchd/systemd). |
| `dropboy sync [path]` | Force an immediate reconciliation pass. |
| `dropboy pause` / `resume` | Temporarily halt sync (e.g. on metered networks). |
| `dropboy restore --machine <hostname> [--into <dir>]` | Pull a remote machine's tree onto this device. |
| `dropboy conflicts` | List unresolved conflicts and resolve interactively. |
| `dropboy doctor` | Diagnose config, AWS perms, clock skew, watcher limits. |
| `dropboy logs [-f]` | Tail daemon logs. |
| `dropboy ui [--port 7777] [--open]` | Start (or focus) the localhost web UI for browsing synced files. |

## 5. Functional Requirements

### 5.1 Sync engine (two-way)
- **Change detection (local)**: filesystem watcher (`fsnotify`) for real-time events plus periodic full-tree scan (default 15 min) to catch missed events (common on macOS Spotlight throttling, network volumes, etc.).
- **Change detection (remote)**: periodic `ListObjectsV2` scan with `If-Modified-Since` / ETag tracking. Default cadence 60s; configurable. (S3 Event Notifications via SQS is a future enhancement that would eliminate polling.)
- **State store**: local SQLite DB (`~/.dropboy/state.db`) recording per-path `{local_mtime, local_size, local_hash, s3_etag, s3_version_id, last_synced_at}`. The DB is the source of truth for "what does each side look like as of the last sync."
- **Reconciliation**: classic three-way compare per path between (last-synced-state, current-local-state, current-remote-state):
  - Local changed, remote unchanged → upload.
  - Remote changed, local unchanged → download.
  - Both unchanged → no-op.
  - Both changed → **conflict** (see 5.4).
  - Local deleted, remote unchanged → delete remote (with grace period; see 5.5).
  - Remote deleted, local unchanged → delete local.

### 5.2 S3 layout
- All objects live under: `s3://<bucket>/dropboy/v1/<machine-id>/<absolute-local-path>`.
  - `<machine-id>` is a stable hostname-derived ID chosen at `init` (overridable). This is what `--machine` in `restore` references.
  - Absolute paths preserve the user's system layout, enabling browsability in the S3 console even though contents are encrypted.
- Sidecar metadata is stored as S3 object metadata (`x-amz-meta-dropboy-*`): plaintext size, plaintext SHA-256, encryption scheme, nonce, original mtime, original mode.
- A manifest file `s3://<bucket>/dropboy/v1/<machine-id>/.dropboy/manifest.json` is updated periodically and used for fast restore listings.

### 5.3 Client-side encryption
- v1 uses **AES-256-GCM** with per-file random nonces.
- A **master key** is derived from a user passphrase via Argon2id at `init`, stored in the OS keychain (Keychain on macOS, Secret Service on Linux, DPAPI on Windows). The passphrase itself is never stored.
- A separate **data encryption key (DEK)** is generated per file, encrypted with the master key, and stored in object metadata (envelope encryption). This enables future key rotation without re-encrypting payloads.
- Filenames in the S3 path are **not encrypted in v1** (deferred — see Open Questions). This preserves browsability and simplifies incremental implementation, at the cost of leaking the directory tree shape to anyone with bucket-read access.

### 5.4 Conflict resolution
- A conflict is recorded when both sides changed since the last sync.
- Default strategy: **keep both**. The losing side is renamed `<name> (conflict from <machine-id> <timestamp>).<ext>` and uploaded; the user is notified via `dropboy status` and OS notification (best-effort).
- `dropboy conflicts` provides an interactive resolver (`keep local` / `keep remote` / `keep both`).
- No silent overwrite is ever performed.

### 5.5 Deletions
- A local delete enters a **soft-delete grace window** (default 24h) before propagating to S3, to protect against accidental `rm -rf`. During the window, the object is moved to `.dropboy/trash/<machine-id>/...` in S3 rather than deleted.
- After the grace window, the trash object is deleted (or transitioned to Glacier — configurable).
- S3 bucket versioning is **recommended** in `dropboy doctor` output but not required.

### 5.6 Daemon
- Single long-running process. Installed as:
  - macOS: `launchd` user agent (`~/Library/LaunchAgents/com.dropboy.plist`).
  - Linux: `systemd --user` unit.
  - Windows: deferred to v1.1.
- Health endpoint on a local Unix socket; `dropboy status` talks to it.
- Resource caps: configurable max upload/download bandwidth, max concurrent transfers (default 4), max file size (default unlimited but warn >5GB).

### 5.7 Configuration
- Single YAML file at `~/.dropboy/config.yaml` plus state DB. Example:
```yaml
bucket: my-dropboy-bucket
region: us-east-1
aws_profile: default
machine_id: golilis-mbp
encryption:
  scheme: aes-256-gcm
  keyring: os
folders:
  - path: /Users/goodylili/Documents
    exclude: ["*.log", ".DS_Store"]
  - path: /Users/goodylili/Projects
    exclude: ["node_modules/**", "vendor/**", ".git/**", "target/**"]
limits:
  max_upload_mbps: 0   # 0 = unlimited
  max_concurrent: 4
  delete_grace_hours: 24
poll:
  remote_seconds: 60
  full_scan_minutes: 15
```

### 5.8 Local web UI
- The daemon embeds an **HTTP server bound to `127.0.0.1`** (default port `7777`, configurable) serving a single-page web UI. Loopback-only — never exposed on `0.0.0.0` or any external interface.
- `dropboy ui` opens the UI in the user's default browser (`--open` flag, on by default when invoked interactively); without flags it just prints the URL.
- **What the UI shows**:
  - A unified browser across **all machines' trees** in the bucket (machine picker at the top: `golilis-mbp`, `desktop`, etc.), rendered as a familiar Finder/Drive-style directory tree.
  - Per-file metadata: size, last modified, originating machine, sync status (synced / pending / conflict / error), encryption indicator.
  - **Preview & download**: clicking a file streams it through the daemon (which decrypts on the fly) — images, PDFs, text, video previewed inline; binaries offered as download.
  - **Sync dashboard**: live queue depth, throughput, recent activity log, current bandwidth usage, paused/active state with toggle.
  - **Conflicts view**: list of unresolved conflicts with side-by-side metadata and one-click `keep local` / `keep remote` / `keep both` actions (same operations as `dropboy conflicts` CLI).
  - **Watched-folder management**: add/remove folders, edit exclude patterns — the same surface as `dropboy add` / `dropboy remove`.
  - **Restore flow**: pick a machine + subtree, choose a destination on this device, kick off `restore` and watch progress.
  - **Settings**: bandwidth caps, poll intervals, grace window, view current bucket/region (read-only display of AWS config).
- **Architecture**: the UI is a static SPA (likely React or Svelte + Vite, TBD) embedded in the Go binary via `embed.FS`. It talks to the daemon via a JSON HTTP API (`/api/v1/*`) and a WebSocket (`/api/v1/events`) for live status, queue, and log streaming. The same API powers both the UI and the `dropboy` CLI (CLI uses the Unix socket; UI uses loopback HTTP).
- **Security**:
  - Loopback bind only; reject any `Host` header that isn't `localhost`/`127.0.0.1` to defeat DNS rebinding.
  - A **session token** is generated at daemon start and written to `~/.dropboy/ui.token` (mode `0600`). The CLI's `dropboy ui` reads it and appends it to the launched URL. All `/api/*` requests must present the token (cookie or `Authorization` header).
  - Strict CSRF protection (token in custom header for state-changing requests), SameSite=Strict cookies, no third-party assets loaded from the page.
  - Decryption happens **inside the daemon**, not in the browser — plaintext bytes are only handed to the browser over loopback. The master key never leaves the daemon process.
- **Non-goals for the UI (v1)**: no in-browser editing of files (read/preview only), no multi-user accounts, no remote access (no Tailscale/Cloudflare-Tunnel-style exposure — explicit non-goal until threat model is revisited).

## 6. Non-Functional Requirements

- **Performance**: idle CPU < 1%, idle RSS < 100 MB. Sync latency for a single small-file save: target < 5 s p50 to S3.
- **Reliability**: at-least-once upload semantics; resumable multipart uploads for files > 16 MB; durable state DB (WAL mode).
- **Security**:
  - All payloads encrypted client-side before leaving the host.
  - AWS credentials sourced from standard SDK chain (env, profile, IMDS). Never stored by dropboy.
  - Master passphrase never logged, never written to disk in plaintext.
- **Observability**: structured logs (slog), `dropboy status` for human view, optional Prometheus exporter (v1.1).
- **Cross-platform**: macOS and Linux at v1; Windows at v1.1.
- **Distribution**: single static Go binary; Homebrew tap; Linux packages (deb/rpm) via goreleaser.

## 7. Architecture (high level)

```
     ┌────────────────┐
│ Watched dirs │ ──────────────► │ Local watcher  │
└──────────────┘                 └───────┬────────┘
                                         │ events
                                         ▼
┌──────────────┐    list/poll    ┌────────────────┐    decisions   ┌────────────────┐
│  S3 bucket   │ ◄────────────►  │ Reconciler     │ ─────────────► │ Transfer queue │
└──────────────┘                 └───────┬────────┘                └───────┬────────┘
                                         │ reads/writes                    │
                                         ▼                                 ▼
                                ┌────────────────┐                ┌────────────────┐
                                │ State DB       │                │ Crypto + S3    │
                                │ (SQLite)       │                │ client         │
                                └────────────────┘                └────────────────┘
```

Components:
- **Watcher**: fsnotify subscriber per watched root; debounces rapid events.
- **Remote poller**: schedules `ListObjectsV2` passes against each machine prefix.
- **Reconciler**: pure function over (state DB, local snapshot, remote snapshot) → list of operations.
- **Transfer worker pool**: executes uploads/downloads with retry and multipart support.
- **Crypto layer**: wraps `io.Reader` streams for AES-GCM with chunked authentication for large files.
- **Control plane**: Unix-socket RPC server for the `dropboy` CLI subcommands.

## 8. Milestones

| Milestone | Scope |
|---|---|
| M0 — Skeleton | Cobra CLI, config, `init`, `add`, `list`. No sync yet. |
| M1 — One-way upload | Watcher + uploader + state DB. Plaintext. macOS only. |
| M2 — Encryption | Client-side AES-GCM, envelope keys, keychain integration. |
| M3 — Two-way sync | Remote poller, reconciler, downloads, conflict handling. |
| M4 — Daemonization | launchd/systemd integration, status RPC, logs. |
| M5 — Restore & deletes | `restore`, soft-delete grace window, trash. |
| M6 — Polish | Bandwidth limits, exclude patterns, `doctor`, packaging, Linux parity. |
| M7 — Local web UI | Embedded SPA, loopback HTTP API, file browser, conflicts view, sync dashboard, restore flow. |
| v1.1 | Windows support, SQS-based remote change notifications, encrypted filenames. |

## 9. Risks

- **fsnotify limitations**: kernel watcher limits on Linux (`fs.inotify.max_user_watches`), event coalescing on macOS, no recursive watches on Linux. Mitigation: full-tree fallback scan; surface limit-bump guidance in `doctor`.
- **Conflict UX**: two-way sync conflicts are the #1 trust killer in this product category. Mitigation: never overwrite silently; conservative defaults; clear notifications.
- **Encryption key loss**: if the user forgets the passphrase and loses the keychain entry, data is unrecoverable. Mitigation: `init` forces the user to confirm they have saved a recovery code (passphrase printed once to stdout).
- **S3 cost surprises**: lots of small files → lots of PUT requests. Mitigation: batch small files in v1.1; surface request-count estimates in `dropboy status`.
- **Clock skew**: reconciliation uses mtimes; bad clocks cause false conflicts. Mitigation: `doctor` checks NTP; hashes are tiebreakers.

## 10. Open Questions

1. **Encrypted filenames / paths**: leaks directory-tree structure today. Encrypt with a deterministic scheme (loses browsability) in v1.1?
2. **Restore granularity**: should `restore` support point-in-time (requires S3 versioning) or always latest in v1?
3. **Shared buckets across users**: multi-user single bucket with per-user prefixes — in scope or explicitly out?
4. **Symlinks, hardlinks, sparse files, extended attributes / ACLs**: which subset does v1 preserve? Proposal: store symlink targets as metadata; ignore hardlinks; preserve mtime + mode only.
5. **Storage class policy**: should dropboy proactively transition cold files to S3 IA/Glacier, or leave that to user-managed bucket lifecycle rules?
6. **Telemetry**: any opt-in anonymous metrics, or strictly zero-phone-home?
