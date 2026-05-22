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

If your default AWS profile has bucket-creation rights, run this once:

```sh
BUCKET=goodylili-dropboy
REGION=eu-west-2

aws s3api create-bucket --bucket "$BUCKET" --region "$REGION" \
  --create-bucket-configuration LocationConstraint="$REGION"

aws s3api put-bucket-versioning --bucket "$BUCKET" \
  --versioning-configuration Status=Enabled

aws s3api put-bucket-encryption --bucket "$BUCKET" \
  --server-side-encryption-configuration \
  '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"},"BucketKeyEnabled":true}]}'

aws s3api put-public-access-block --bucket "$BUCKET" \
  --public-access-block-configuration \
  BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
```

This gives you a private, versioned, SSE-S3-at-rest bucket. dropboy ALSO
encrypts every object client-side under your own master key — SSE-S3 is
defence in depth and ensures the bytes are encrypted on AWS's disks even
before dropboy's payload hits.

## 3. AWS credentials

Create a dedicated IAM user `dropboy-laptop` with a least-privilege policy.
**Most CLI users don't have `iam:CreateUser`** — if that's you, do this part
in the AWS console:

1. IAM → Users → **Create user** → name `dropboy-laptop`, no console access.
2. **Attach policies directly** but tick nothing, then **Create user**.
3. Open the user → **Permissions** → **Add permissions** → **Create inline
   policy** → switch to JSON and paste:

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

   Name it `dropboy-bucket-access`.
4. **Security credentials** tab → **Create access key** → choose
   **Command Line Interface (CLI)** → finish. **Copy the secret now** —
   AWS won't show it again.

Add the profile to `~/.aws/credentials`:

```sh
aws configure --profile dropboy
# Access key ID:  AKIA...
# Secret:         ...
# Region:         eu-west-2
# Output format:  json
```

Sanity-check the policy with a put/get/delete round-trip:

```sh
aws --profile dropboy sts get-caller-identity        # → arn:.../user/dropboy-laptop
echo hi > /tmp/check && aws --profile dropboy s3 cp /tmp/check s3://goodylili-dropboy/_permcheck \
  && aws --profile dropboy s3 rm s3://goodylili-dropboy/_permcheck
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
- Passphrase → accept the generated one (recommended) or type your own.
  Stored in the macOS Keychain after the daemon starts, so you only need
  it on first run or new machines.
- Recovery code → printed once at the end. **Save it in a password manager
  or somewhere durable.** It's an independent secret that wraps the same
  master key; if you ever forget the passphrase, `dropboy recover` will
  unlock with this. You only lose data if you lose BOTH.

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

```sh
echo "hello $(date)" > ~/Documents/dropboy-hello.txt
sleep 5
dropboy status
aws --profile dropboy s3 ls s3://goodylili-dropboy/dropboy/v1/ --recursive
```

You should see the object listed. Confirm it's actually ciphertext (not just
"encrypted in transit"):

```sh
aws --profile dropboy s3 cp s3://goodylili-dropboy/dropboy/v1/<machine>/Users/goodylili/Documents/dropboy-hello.txt - | xxd | head -2
```

Random bytes, not your message. The plaintext hash, scheme, and wrapped DEK
are in the object's user metadata (visible via `aws s3api head-object`).

Round-trip restore from a different working directory:

```sh
dropboy restore --machine <machine-id> --into /tmp/restore-test
diff ~/Documents/dropboy-hello.txt /tmp/restore-test/.../dropboy-hello.txt
```

### Test the recovery path

Worth doing once so you know it works before you need it:

```sh
# Stop the daemon and pretend the passphrase + keychain are gone.
dropboy stop
security delete-generic-password -s com.dropboy -a <machine-id> 2>/dev/null

# Start without DROPBOY_PASSPHRASE → the daemon boots LOCKED (UI up, engine off).
dropboy start --foreground &
dropboy status                # shows daemon up, engine offline / locked

# Recover with the code you saved at init.
dropboy recover               # paste recovery code → unlocked for this session
dropboy status                # engine back online
```

If recovery worked, also re-seed the keychain so future boots are seamless:
open the UI (`dropboy ui --open`) and re-enter your passphrase with "remember
on this machine" ticked, OR set `DROPBOY_PASSPHRASE` and restart the daemon
once.

## 7. Add a second machine (later)

The master key is randomly generated at `dropboy init` — the passphrase only
unwraps it. So a second machine can't just "use the same passphrase" and
read machine A's encrypted files; it would generate its own master key and
fail to decrypt them.

Until cross-machine key sync is wired up (TODO), the manual flow is:

```sh
# On machine A:
cp ~/.dropboy/master.key ~/.dropboy/master.recovery.key /tmp/  # copy securely

# On machine B (after install, BEFORE dropboy init):
mkdir -p ~/.dropboy && cp /tmp/master.key /tmp/master.recovery.key ~/.dropboy/
chmod 600 ~/.dropboy/master.key ~/.dropboy/master.recovery.key

# Now dropboy init on B will detect the existing master key, skip key
# generation, and use the same passphrase as A.
dropboy init       # use a DIFFERENT machine_id but the SAME passphrase
```

Each machine still syncs its own prefix (`dropboy/v1/<machine-id>/...`).
Use `dropboy restore --machine <other-machine-id>` to pull another
machine's tree onto this one — works because B now holds A's master key.

## Common gotchas

- **`dropboy status` says locked after reboot.** Keychain prompt may have
  been dismissed — `dropboy ui` and re-enter your passphrase, or set
  `DROPBOY_PASSPHRASE` and `dropboy start --foreground` once to seed it.
- **Forgot the passphrase.** Run `dropboy recover` and paste the recovery
  code you saved at init. That unlocks the engine for this session; set a
  new passphrase via the UI to make boots seamless again.
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
