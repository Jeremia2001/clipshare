# ClipShare Admin Guide

This guide walks a self-hosting operator through standing up a ClipShare server, creating the admin account, inviting users, and configuring the storage limit.

---

## 1. What you need

- A host with Docker + Docker Compose
- A reachable hostname or LAN IP (users will point the desktop app at this)
- ~5 min of uninterrupted terminal access for first launch (to read the setup token from logs)

The stack runs three containers:

| Service | Purpose |
|---|---|
| `postgres` | Metadata: users, clips, invites, device tokens |
| `rustfs` | Object storage for raw + processed clips and thumbnails |
| `api` | The Go API server the desktop clients talk to |

---

## 2. Configure the server

Copy the example env file and edit the secrets:

```bash
cd docker
cp .env.example .env
# edit docker/.env
```

For a real deployment, set at least:

| Env var | What to set |
|---|---|
| `ENV` | `production` (disables the seeded dev user and enables the setup-token flow) |
| `JWT_SECRET` | A long random string — production refuses to start without this |
| `DB_PASSWORD` | A real Postgres password |
| `RUSTFS_ACCESS_KEY`, `RUSTFS_SECRET_KEY` | Real RustFS credentials |
| `SERVER_PUBLIC_URL` | The external URL users will reach the API at, e.g. `https://clips.example.com` |

Generate a secret quickly:

```bash
openssl rand -base64 48
```

Start the stack:

```bash
docker compose up -d
```

Compose automatically picks up `docker/.env`. Migrations run on container start.

---

## 3. Grab the one-time setup token

On first launch, the server sees no admin in the database and mints a one-time setup token. It's printed to stdout — read it from the container logs:

```bash
docker compose logs api | grep -A 4 "setup token"
```

You'll see a block like:

```
========================================================
 ClipShare admin setup required.
 Use this one-time setup token to create the admin account:
   AB12-CD34-EF56-GH78
 This token is single-use and will not be shown again.
========================================================
```

Copy the token. It's consumed the moment you use it to create the admin, after which it's wiped.

**Lost it?** Restart the API container — if no admin has been created yet, the server rotates the old unused token and prints a fresh one on the next boot.

---

## 4. Create the admin account

On your workstation, launch the ClipShare desktop app. The login screen offers three modes:

1. **First-time admin setup** — pick this.
   - Server URL: the public URL you set as `SERVER_PUBLIC_URL`
   - Username: your admin login handle (ClipShare doesn't send email — pick any handle you like)
   - Password: pick a strong one
   - Setup token: paste the token from the server logs

Hit **Create admin**. The desktop app stores a JWT and you land in the main UI. You're now the admin.

Once this step succeeds, the setup endpoint is locked — future clients must either log in as the admin or redeem an invite.

---

## 5. Invite users

All admin controls live in **Settings → Invite codes** in the desktop app.

1. Click **Create invite**.
2. Optionally add a note ("for alice") and an expiry in days.
3. The code appears once, e.g. `K7N-PQ9R`. Copy it now — only the hash is stored server-side, so you can't read it again later.
4. Send the user two things:
   - The **server URL** (same one you used)
   - The **invite code**

On their desktop app, the user picks **Join with invite** and enters:

- Server URL
- A username (they pick it — it's their display handle)
- The invite code
- An optional device label ("desk PC")

The server:

- Creates their user record if the username is new
- Atomically marks the invite redeemed (single-use — a race between two users loses cleanly)
- Issues a **device token** and returns it to the client

The desktop app stores the device token in the OS keyring (DPAPI on Windows, Keychain on macOS, Secret Service on Linux) keyed to the server URL. The user never types a password again on that machine.

### Rules to know

- **One user = one device.** If the same username tries to redeem a second invite, the server rejects it with *"this account already has a registered device"*. For a second machine, revoke the first device (see below) or have the user join with a different username.
- **Invites are single-use.** Redeemed or expired codes cannot be reused; delete them from the list to keep it tidy.
- **Expiry is optional.** Leave the days field empty for a code that never expires until redeemed.

### Revoke a device

Invite management shows who redeemed which code. To kick a device off the server, deleting the device token row from `device_tokens` is enough — a CLI command for this is not wired up yet, so for now run:

```bash
docker compose exec postgres psql -U clipshare -c \
  "DELETE FROM device_tokens WHERE user_id = (SELECT id FROM users WHERE username = 'alice');"
```

The user can then redeem a new invite on the same username.

---

## 6. Configure the storage limit

In **Settings → Server storage**, admins see a number input and a GB/MB selector.

- **0** means unlimited — the server will accept uploads until the disk is physically full.
- Any positive number caps the sum of `file_size_bytes` across all clips. When accepting a new upload would push the total over the limit, the API returns **HTTP 507 Insufficient Storage** and the desktop app shows a clear error.
- All users see the current usage as a progress bar, so they know when the server is filling up.

Raising or lowering the limit is a live change — no restart required. Lowering it below current usage doesn't delete anything; it just blocks new uploads until usage drops.

---

## 7. Day-to-day operations

| Task | Where |
|---|---|
| View/revoke invites | Desktop app → Settings → Invite codes |
| Set storage cap | Desktop app → Settings → Server storage |
| Check server logs | `docker compose logs -f api` |
| Back up data | Snapshot the `postgres_data` and `rustfs_data` Docker volumes together |
| Upgrade | `docker compose pull && docker compose up -d` — migrations run on boot |

---

## 8. Troubleshooting

**"Invalid setup token"** — Token was already consumed, or you copied whitespace. Restart the `api` container to mint a new one (only possible while no admin exists).

**"Invalid or expired invite code"** — Check the invite list in Settings; the code may be redeemed, expired, or deleted. Generate a fresh one.

**"This account already has a registered device"** — The user previously joined from another machine. Revoke the old device token (see §5) or use a different username.

**HTTP 507 on upload** — Server storage limit hit. Raise the limit or delete old clips.

**Server won't start: "JWT_SECRET must be set in production"** — You're running with `ENV=production` but left the default secret. Set a real `JWT_SECRET`.
