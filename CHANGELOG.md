# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-19

Initial macOS-only release.

### Added
- Two-way sync engine with three-way reconciliation, conflict detection, and
  SQLite-backed state store.
- Client-side AES-256-GCM envelope encryption with Argon2id-derived KEK.
- macOS Keychain integration for passphrase persistence.
- Loopback HTTP API (`127.0.0.1:7777`) with session token, CSRF protection,
  DNS-rebinding host guard, security headers, rate limiting on passphrase
  endpoints, and 8 MiB request body cap.
- Server-Sent Events stream for live status updates.
- Next.js 16 web UI, statically exported and embedded into the Go binary.
- launchd service install/start/stop via `dropboy start` / `stop` / `restart`.
- CLI: `init`, `add`, `remove`, `list`, `status`, `sync`, `pause`, `resume`,
  `restore`, `conflicts`, `doctor`, `logs`, `ui`, `version`.
- Bandwidth throttling via `limits.max_upload_mbps`.
- AWS SDK adaptive retry on S3 calls.
- Log rotation to `~/Library/Logs/dropboy/dropboy.log` (10 MiB × 3 files).
- Homebrew tap for `brew install goodylili/tap/dropboy`.

[Unreleased]: https://github.com/goodylili/dropboy/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/goodylili/dropboy/releases/tag/v0.1.0
