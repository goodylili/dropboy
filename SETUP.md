# Setting up dropboy on your own machine

Quick path for the maintainer (you, goodylili) doing a from-source install on
your laptop. For end-user docs, see [`README.md`](README.md).

## 1. Build and install

```sh
make release-build      # builds the frontend, embeds it, builds the binary
make install            # installs to /usr/local/bin/dropboy (or ~/.local/bin)
dropboy version
```

Requirements: Go 1.25+, Node 20+, macOS.

The Makefile injects version metadata via `-ldflags` from
`git describe --tags --always --dirty`, the short commit, and the build
timestamp. On an untagged tree you'll see something like
`dropboy 0f5a2d1-dirty`; to cut a clean version, tag first:

```sh
git tag v0.1.0 && make release-build   # -> dropboy v0.1.0
```

You can also override explicitly: `make release-build VERSION=v0.1.0`.

## 2. Create your S3 bucket

In the AWS console (or `aws s3api`):

1. Create a bucket — e.g. `goodylili-dropboy`. Pick a region close to you
   (e.g. `eu-west-2`).
2. Enable **bucket versioning** (Properties → Bucket Versioning → Enable).
   Lets you restore prior versions.
3. Enable **default encryption**: SSE-S3 is fine; SSE-KMS if you want a CMK.
   (dropboy also encrypts client-side — this is defence in depth.)
4. Block all public access (default — leave on).

## 3. AWS credentials

Create an IAM user `dropboy-laptop` with a policy scoped to the bucket:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "s3:GetObject", "s3:PutObject", "s3:DeleteObject",
      "s3:ListBucket",
      "s3:GetObjectVersion", "s3:ListBucketVersions"
    ],
    "Resource": [
      "arn:aws:s3:::goodylili-dropboy",
      "arn:aws:s3:::goodylili-dropboy/*"
    ]
  }]
}
```

Generate an access key and add it to `~/.aws/credentials` under a named
profile:

```ini
[dropboy]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
region = eu-west-2
```

## 4. First-run

```sh
dropboy init
```

Walks you through:

- Bucket name → `goodylili-dropboy`
- Region → `eu-west-2`
- AWS profile → `dropboy`
- Machine ID → defaults to your hostname; keep it stable
- Passphrase → **write this down**. If you lose it and your Keychain entry,
  the data is unrecoverable. The Argon2id-derived master key wraps every
  per-file DEK.

Add the folders you want synced:

```sh
dropboy add ~/Documents
dropboy add ~/GolandProjects --exclude 'node_modules/**' --exclude 'vendor/**' --exclude '.git/**' --exclude 'target/**' --exclude 'dist/**'
dropboy add ~/Desktop --exclude '.DS_Store'
```

Skip giant build trees — every file is a PUT and you pay per request.

## 5. Start the daemon

```sh
dropboy start           # installs the launchd agent and starts it
dropboy status          # should show running + queue depth
dropboy ui --open       # opens the local web UI on http://127.0.0.1:7777
```

Tail logs if anything looks off:

```sh
dropboy logs -f
# or directly: tail -F ~/Library/Logs/dropboy/dropboy.log
```

## 6. Verify it works

1. Save a small file into a watched folder.
2. `dropboy status` should show queue depth jump and then drop to zero.
3. In the AWS console, browse to
   `s3://goodylili-dropboy/dropboy/v1/<machine-id>/Users/goodylili/...`
   — the object should be there, ciphertext only.
4. `dropboy restore --machine <machine-id> --into /tmp/restore-test` and
   confirm a file round-trips correctly.

## 7. Add a second machine (later)

On the new laptop: install dropboy, point it at the same bucket with a
**different** `machine_id`, enter the same passphrase. Each machine syncs
its own prefix (`dropboy/v1/<machine-id>/...`); the UI shows all of them.
Use `dropboy restore --machine <other-machine-id>` to pull another
machine's tree onto this one.

## Common gotchas

- **`dropboy status` says locked after reboot.** Keychain prompt may have
  been dismissed — `dropboy ui` and re-enter your passphrase, or set
  `DROPBOY_PASSPHRASE` and `dropboy start --foreground` once to seed it.
- **Service won't stay running.** Almost always AWS creds. Check the
  profile name in `~/.dropboy/config.yaml` matches `~/.aws/credentials`,
  and that the access key isn't disabled.
- **Lots of conflicts immediately.** Probably clock skew between machines
  or a folder synced on two machines that diverged before dropboy was set
  up. Resolve in the UI's Conflicts tab.
- **`fs.inotify` warnings on Linux.** Doesn't apply yet — Linux is v1.1.

## Reset everything

```sh
dropboy uninstall
rm -rf ~/.dropboy
# bucket contents are untouched — delete them manually if you want
```
