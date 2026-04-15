# ClipShare - AGENTS.md

Project overview and conventions for AI agents working on this codebase.

## Project Summary

ClipShare is an open-source alternative to Medal.tv for recording, editing, and sharing gaming clips. It follows a self-hosted architecture with a Go API server, a Wails desktop app, and S3-compatible object storage.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend API | Go 1.26 with [Fiber v2](https://docs.gofiber.io/) |
| Database | PostgreSQL 15 (migrations via [Goose v3](https://github.com/pressly/goose)) |
| Caching/Sessions | Redis 7 |
| Object Storage | RustFS (S3-compatible, Apache 2.0 — NOT MinIO due to AGPL) |
| Desktop App | Wails v2 with React 18 + TypeScript |
| Frontend Build | Vite + Tailwind CSS |
| Video Processing | Native FFmpeg (Go-side, bundled with releases) |
| Containerization | Docker Compose |

## Repository Structure

```
clipshare/
├── apps/
│   ├── server/                          # Go Backend (module: clipshare)
│   │   ├── cmd/
│   │   │   ├── api/main.go             # Server entry point (Fiber app, wires all deps)
│   │   │   └── migrate/main.go        # Database migration runner (Goose)
│   │   ├── internal/
│   │   │   ├── config/config.go        # ENV-based config (all settings from env vars)
│   │   │   ├── handlers/auth.go        # HTTP handlers for auth endpoints
│   │   │   ├── middleware/auth.go      # AuthMiddleware, RequireAuth, RequireAdmin, CORS
│   │   │   ├── models/                 # Data models (user.go, clip.go, social.go)
│   │   │   ├── repository/             # DB access layer (user.go, others)
│   │   │   ├── services/auth.go        # Business logic for authentication
│   │   │   └── storage/rustfs.go       # RustFS S3 client (MinIO SDK, presigned URLs)
│   │   ├── pkg/
│   │   │   ├── auth/jwt.go             # JWT token creation/validation
│   │   │   └── email/service.go        # SMTP email sender with templates
│   │   ├── migrations/                  # SQL migrations (001_initial, 002_clips, 003_social)
│   │   ├── Dockerfile                   # Multi-stage Docker build
│   │   └── go.mod                       # Go module (clipshare)
│   │
│   └── desktop/                         # Wails Desktop App (module: clipshare-desktop)
│       ├── frontend/
│       │   ├── src/
│       │   │   ├── components/Layout.tsx     # Sidebar navigation (forest theme)
│       │   │   ├── hooks/useAuth.tsx         # Auth context — calls IsDevMode() via Wails
│       │   │   ├── services/api.ts           # Axios API client
│       │   │   ├── pages/
│       │   │   │   ├── LoginPage.tsx          # Magic link login form
│       │   │   │   ├── LibraryPage.tsx        # Clip library (grid/list, search)
│       │   │   │   ├── EditorPage.tsx         # Video editor, native ffmpeg trim via Wails bindings
│       │   │   │   └── SettingsPage.tsx       # Sectioned settings, dev indicator
│       │   │   ├── styles/index.css          # Tailwind base + custom utility classes
│       │   │   ├── App.tsx                    # Routes with ProtectedRoute / AuthRoute guards
│       │   │   └── main.tsx                   # Entry point
│       │   ├── wailsjs/go/main/
│       │   │   ├── App.js                    # Wails JS bindings (manually maintained)
│       │   │   └── App.d.ts                   # TypeScript declarations for bindings
│       │   ├── tailwind.config.js             # Custom color palettes: forest, earth, sand, moss
│       │   ├── postcss.config.js
│       │   ├── index.html
│       │   ├── package.json
│       │   └── vite.config.ts
│       ├── internal/
│       │   ├── config/config.go          # DevMode check (ENV=development || CLIPSHARE_DEV=1)
│       │   └── ffmpeg/ffmpeg.go           # Native ffmpeg/ffprobe wrapper (probe, trim, thumbnail)
│       ├── main.go                             # Wails app setup, ffmpeg methods, file dialog, local file serving
│       ├── wails.json
│       └── go.mod                               # Go module (clipshare-desktop)
│
├── docker/
│   └── docker-compose.yml               # PostgreSQL, Redis, RustFS, API
│
└── .env.example                         # All environment variables
```

### Video Processing (Native FFmpeg)

The desktop app uses **native ffmpeg** via Go's `os/exec` package, not ffmpeg.wasm. This provides:
- Much faster trimming and encoding (native binary vs WASM)
- Hardware acceleration support when available
- Full ffmpeg/ffprobe feature set

**How it works:**
1. `internal/ffmpeg/ffmpeg.go` wraps `ffmpeg`/`ffprobe` as Go functions
2. `main.go` exposes `FFmpegIsAvailable()`, `ProbeVideo()`, `TrimVideo()`, `OpenFileDialog()` as Wails bindings
3. The frontend calls these Go methods via `wailsjs/go/main/App` bindings
4. Local video files are served to the `<video>` element via `/local-files/` middleware in the Wails asset server
5. `scripts/download-ffmpeg.sh` downloads static ffmpeg binaries for bundling
6. `build.sh` copies ffmpeg binaries alongside the built executable

**FFmpeg discovery**: The Go code first looks for `ffmpeg`/`ffprobe` next to the binary, then falls back to `PATH`. Bundled releases ship ffmpeg alongside the binary.

### Local File Serving

The Wails asset server middleware intercepts `/local-files/` requests and serves the actual filesystem path from the URL:
- `/local-files/home/user/video.mp4` → serves `/home/user/video.mp4`
- This allows the `<video>` element to play local files picked via `OpenFileDialog()`
- `Cache-Control: no-store` is set to prevent caching of local files

## Key Architecture Patterns

### Authentication (Magic Links)
- **Flow**: User enters email → server sends magic link → user clicks link → server verifies token → returns JWT access + refresh tokens
- **Dev mode bypass**: When `ENV=development`, `RequireAuth` middleware skips all auth and sets mock user (`dev-user-id`, `dev@localhost`, admin=true)

### Dev Mode Detection (Important)
- **Backend**: `os.Getenv("ENV") == "development"` in `RequireAuth` middleware
- **Desktop Go**: `config.Load()` checks `ENV` or `CLIPSHARE_DEV` env vars, sets `DevMode` bool
- **Desktop Frontend**: `useAuth` hook calls `IsDevMode()` via Wails JS binding at runtime; if true, auto-sets mock dev user
- **Critical**: `import.meta.env.DEV` is always `false` in Wails production builds — never use Vite env vars for runtime dev detection
- **Wails JS bindings** (`wailsjs/go/main/App.js`, `App.d.ts`) must be manually kept in sync with Go methods on the App struct

### API Routes
All API routes are under `/api/v1`. Auth routes are registered by `authHandler.RegisterRoutes(api)`.

### Database Migrations
Migrations are in `apps/server/migrations/` as numbered SQL files. Run with:
```bash
cd clipshare/apps/server
go run cmd/migrate/main.go up
```

### Frontend Routing
- `/login` — AuthRoute (redirects away if already authenticated)
- `/` — ProtectedRoute → LibraryPage
- `/editor` — ProtectedRoute → EditorPage
- `/settings` — ProtectedRoute → SettingsPage

## Design Language

**Dark green/earth-tone palette** — NOT generic dark mode with indigo/purple.

| Palette | Usage |
|---|---|
| `forest` (50–950) | Primary backgrounds, sidebar, active states |
| `earth` (50–950) | Accent highlights, warnings, dev badge |
| `sand` (50–950) | Text colors, borders, muted elements |
| `moss` (50–950) | Secondary accents, success states |

Custom CSS classes in `styles/index.css`: `.btn-primary`, `.btn-secondary`, `.btn-ghost`, `.card`, `.input-field`, `.section-header`.

## Build & Run Commands

### Infrastructure
```bash
cd clipshare/docker
docker-compose up -d
```
Starts: PostgreSQL (5432), Redis (6379), RustFS (9000/9001), API (8080)

### Server (development)
```bash
cd clipshare/apps/server
go run cmd/api/main.go
```

### Database Migrations
```bash
cd clipshare/apps/server
go run cmd/migrate/main.go up
```

### Desktop App
```bash
cd clipshare/apps/desktop
npm install --prefix frontend
wails dev -tags webkit2_41          # Development
wails build -tags webkit2_41        # Production build (Ubuntu 24.04)
```

The `webkit2_41` build tag is required for Ubuntu 24.04 compatibility.

**Linux system dependency**: WebKitGTK uses GStreamer for media decoding. H.264/MP4 playback requires codec plugins that are not installed by default on many Linux distributions. If video shows a black screen with controls, install:
```bash
sudo apt-get install -y gstreamer1.0-libav gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly
```

## Environment Variables

All config is via environment variables. See `.env.example` for the full list. Key variables:

| Variable | Default | Purpose |
|---|---|---|
| `ENV` | `development` | Controls dev mode auth bypass and desktop DevMode |
| `CLIPSHARE_DEV` | — | Alternative dev mode flag for desktop app |
| `SERVER_HOST` | `0.0.0.0` | API server bind address |
| `SERVER_PORT` | `8080` | API server port |
| `DB_HOST/PORT/USER/PASSWORD/NAME` | localhost/5432/clipshare | PostgreSQL config |
| `REDIS_HOST/PORT` | localhost/6379 | Redis config |
| `RUSTFS_ENDPOINT` | `localhost:9000` | RustFS S3 endpoint |
| `JWT_SECRET` | (change in prod) | JWT signing key |
| `SMTP_*` | — | Email service for magic links |

## Storage Architecture

RustFS uses three S3 buckets:
- `clips-raw` — Original uploaded clips
- `thumbnails` — Generated thumbnails
- `clips-processed` — Transcoded/processed clips

The `storage/rustfs.go` client uses the MinIO Go SDK (which is S3-compatible) to communicate with RustFS.

## Current Status

### Completed (Phase 1)
- Go server with Fiber framework
- Magic link authentication (JWT tokens, refresh tokens)
- PostgreSQL database with Goose migrations (3 migration files)
- Redis integration for sessions
- RustFS S3 client (presigned URL generation ready)
- Wails desktop app scaffold with React
- Frontend pages: Login, Library, Editor, Settings
- Docker-compose for local development
- Dev mode auth bypass (backend middleware + desktop app)
- UI redesign with forest/earth/sand/moss color palette
- Native ffmpeg integration for video trimming (Go-side via `os/exec`)
- Local file serving via Wails asset middleware for video preview
- File dialog for video selection (`runtime.OpenFileDialog`)
- ffmpeg/ffprobe probe, trim with progress, thumbnail generation

### Not Yet Implemented (Phase 2+)
- Clip editor UI with timeline
- Clip recording functionality
- Clip sharing and social features
- Thumbnail generation pipeline

## Conventions

- **Go**: Standard project layout (`cmd/`, `internal/`, `pkg/`). Module name `clipshare` for server, `clipshare-desktop` for desktop app.
- **Frontend**: React functional components with hooks. State management via React context (`useAuth`). Styling via Tailwind utility classes and custom CSS classes.
- **Routing**: Frontend uses `react-router-dom` with `Routes`/`Route`. Auth guards are `ProtectedRoute` and `AuthRoute` components.
- **API Client**: Axios instance in `services/api.ts` with base URL from config.
- **Icons**: lucide-react icon library. (Note: `FilmScissors` doesn't exist; use `Scissors` instead.)
- **No comments in code** unless explicitly requested.
- **Never use MinIO** — always use RustFS for S3-compatible storage due to AGPL license concerns.