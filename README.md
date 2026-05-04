# ClipShare

Self-hosted clipboard sharing. Windows client (macOS and Linux planned) connect to your own server — no cloud, no accounts, no telemetry.

---

## Stack

| Service | Purpose |
|---|---|
| `postgres` | Metadata: users, clips, invites, device tokens |
| `rustfs` | S3-compatible object storage for clips and thumbnails (local profile only) |
| `api` | Go API server the desktop clients talk to |

---

## Server Setup

### 1. Configure environment

```bash
cd docker
cp .env.example .env
# edit docker/.env
```

Key variables for production:

| Variable | Description |
|---|---|
| `ENV` | Set to `production` — enables the setup-token flow and disables the dev seed user |
| `JWT_SECRET` | Long random string — server refuses to start in production without it |
| `DB_PASSWORD` | Postgres password |
| `RUSTFS_ACCESS_KEY`, `RUSTFS_SECRET_KEY` | RustFS credentials |
| `SERVER_PUBLIC_URL` | External URL clients will reach, e.g. `https://clips.example.com` |

Generate a secret:

```bash
openssl rand -base64 48
```

### 2. Start the stack

**Local dev** (includes RustFS container for object storage):

```bash
docker compose --profile local up -d
```

**Production** (uses external S3-compatible storage — set `RUSTFS_ENDPOINT` and credentials in `.env`):

```bash
docker compose up -d
```

Compose reads `docker/.env` automatically. Migrations run on container start.

The `rustfs` service is gated behind the `local` profile. Without `--profile local`, only `postgres` and `api` start.

### 3. Grab the one-time setup token

On first launch the server sees no admin and mints a one-time setup token printed to stdout:

```bash
docker compose logs api | grep -A 4 "setup token"
```

Output:

```
========================================================
 ClipShare admin setup required.
 Use this one-time setup token to create the admin account:
   AB12-CD34-EF56-GH78
 This token is single-use and will not be shown again.
========================================================
```

Copy the token — it's consumed the moment you create the admin account. If you lose it, restart the `api` container; a fresh token is minted as long as no admin exists.

### 4. Create the admin account

Launch the ClipShare desktop app and choose **First-time admin setup**:

- **Server URL** — your `SERVER_PUBLIC_URL`
- **Username** — your admin handle (no email required)
- **Password** — pick a strong one
- **Setup token** — paste from logs

Hit **Create admin**. The setup endpoint locks permanently after this step.

---

## Inviting Users

All admin controls are in **Settings → Invite codes** in the desktop app.

1. Click **Create invite**, optionally add a note and expiry in days.
2. Copy the code (e.g. `K7N-PQ9R`) — only the hash is stored server-side, so you can't read it again.
3. Send the user the **server URL** and the **invite code**.

The user picks **Join with invite** and enters server URL, a username, the invite code, and an optional device label. The server creates their account, marks the invite redeemed, and issues a device token stored in the OS keyring (DPAPI / Keychain / Secret Service). No password needed on subsequent logins from that machine.

**Rules:**
- One user = one device. A second invite redemption on the same username is rejected. Revoke the first device or use a different username.
- Invites are single-use. Redeemed or expired codes can't be reused.

**Revoke a device:**

```bash
docker compose exec postgres psql -U clipshare -c \
  "DELETE FROM device_tokens WHERE user_id = (SELECT id FROM users WHERE username = 'alice');"
```

The user can then redeem a new invite on the same username.

---

## Storage Limit

**Settings → Server storage** in the desktop app.

- **0** = unlimited (accepts uploads until disk is full)
- Positive number caps total `file_size_bytes` across all clips. Exceeding it returns HTTP 507; the desktop app shows a clear error.
- All users see current usage as a progress bar.
- Changes are live — no restart required. Lowering below current usage blocks new uploads but doesn't delete existing clips.

---

## Day-to-Day Operations

| Task | Where |
|---|---|
| View/revoke invites | Desktop app → Settings → Invite codes |
| Set storage cap | Desktop app → Settings → Server storage |
| Check server logs | `docker compose logs -f api` |
| Back up data | Snapshot `postgres_data` and `rustfs_data` Docker volumes together |
| Upgrade | `docker compose pull && docker compose up -d` — migrations run on boot |

---

## Troubleshooting

**"Invalid setup token"** — Token already consumed or you copied whitespace. Restart the `api` container to mint a new one (only possible while no admin exists).

**"Invalid or expired invite code"** — Code may be redeemed, expired, or deleted. Generate a fresh one in Settings.

**"This account already has a registered device"** — Revoke the old device token (see above) or use a different username.

**HTTP 507 on upload** — Storage limit hit. Raise the limit or delete old clips.

**"JWT_SECRET must be set in production"** — Running with `ENV=production` but left the default secret. Set a real `JWT_SECRET`.

---

## Building the Desktop App

```bash
# Install frontend dependencies
npm install --prefix apps/desktop/frontend

# Build (Ubuntu 24.04 needs webkit2_41 tag)
cd apps/desktop
wails build -tags webkit2_41

# Dev mode
wails dev -tags webkit2_41
```

---

## License

[GNU Affero General Public License v3.0](LICENSE) — see [LICENSE](LICENSE) for full terms.
