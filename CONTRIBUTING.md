# Contributing to dropboy

Thanks for your interest! This is a single-maintainer project at the moment,
so please open an issue before sending a non-trivial PR.

## Dev setup

You'll need:

- Go 1.25+
- Node.js 20+
- macOS (Apple Silicon or Intel) — Linux/Windows are not currently supported

```sh
git clone https://github.com/goodylili/dropboy
cd dropboy
make build           # frontend export + embed + go build
./bin/dropboy --help
```

## Day-to-day loop

```sh
make frontend-dev    # Next.js dev server on :3000
make build-go        # Go binary only (no frontend rebake)
./bin/dropboy start --foreground
```

The dev server proxies `/api/*` to the daemon on `127.0.0.1:7777`.

## Before you push

```sh
make fmt vet test
```

CI runs the same on macos-latest plus a frontend lint + static export build.

## Tests

- Go tests live next to their packages (`_test.go`).
- Frontend currently has no test runner wired up; lint via `npm run lint`.
- Avoid network-dependent tests — S3 interactions are mocked at the
  `s3.Client` interface boundary.

## Commit messages

Conventional commits are nice-to-have but not enforced. Keep subject lines
under 72 chars and write the body for someone reading `git log` in two
years.

## Releases

Maintainer tags `vX.Y.Z`; the `release` workflow handles goreleaser →
Homebrew tap.
