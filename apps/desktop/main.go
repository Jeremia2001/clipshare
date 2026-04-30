package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientauth "clipshare-desktop/internal/auth"
	"clipshare-desktop/internal/config"
	"clipshare-desktop/internal/ffmpeg"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

type mediaEntry struct {
	path      string
	deletable bool // true for trimmed outputs, false for original source files
}

type streamEntry struct {
	url   string
	token string
}

type App struct {
	ctx             context.Context
	config          *config.Config
	mediaDir        string
	mediaMu         sync.RWMutex
	mediaMap        map[string]mediaEntry
	mediaSeq        int64 // monotonic counter; never resets so IDs are always unique
	streamMu        sync.Mutex
	streamMap       map[string]streamEntry
	streamCounter   int
	mediaServerAddr string
	mediaServer     *http.Server
}

func NewApp() *App {
	return &App{
		mediaMap:  make(map[string]mediaEntry),
		streamMap: make(map[string]streamEntry),
	}
}

func (a *App) mediaSet(name, path string, deletable bool) {
	a.mediaMu.Lock()
	a.mediaMap[name] = mediaEntry{path: path, deletable: deletable}
	a.mediaMu.Unlock()
}

func (a *App) mediaGet(name string) (string, bool) {
	a.mediaMu.RLock()
	e, ok := a.mediaMap[name]
	a.mediaMu.RUnlock()
	return e.path, ok
}

func (a *App) mediaDel(name string) (mediaEntry, bool) {
	a.mediaMu.Lock()
	e, ok := a.mediaMap[name]
	if ok {
		delete(a.mediaMap, name)
	}
	a.mediaMu.Unlock()
	return e, ok
}


func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}
	a.config = cfg

	a.mediaDir, err = os.MkdirTemp("", "clipshare-media-*")
	if err != nil {
		fmt.Printf("Failed to create media temp dir: %v\n", err)
		os.Exit(1)
	}

	if err := a.startMediaServer(); err != nil {
		fmt.Printf("Failed to start media server: %v\n", err)
		os.Exit(1)
	}
}

func (a *App) startMediaServer() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	a.mediaServerAddr = ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/media/")

		resolved, ok := a.mediaGet(name)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		f, err := os.Open(resolved)
		if err != nil {
			http.Error(w, "open failed", http.StatusNotFound)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, "stat failed", http.StatusInternalServerError)
			return
		}

		mimeType := mime.TypeByExtension(filepath.Ext(resolved))
		if mimeType == "" {
			mimeType = "video/mp4"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeContent(w, r, info.Name(), time.Time{}, f)
	})

	mux.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		key := strings.TrimPrefix(r.URL.Path, "/stream/")

		a.streamMu.Lock()
		entry, ok := a.streamMap[key]
		a.streamMu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, entry.url, nil)
		if err != nil {
			http.Error(w, "bad upstream url", http.StatusInternalServerError)
			return
		}
		if entry.token != "" && !isPresignedS3URL(entry.url) {
			upstream.Header.Set("Authorization", "Bearer "+entry.token)
		}
		if rng := r.Header.Get("Range"); rng != "" {
			upstream.Header.Set("Range", rng)
		}

		client := &http.Client{Timeout: 0}
		resp, err := client.Do(upstream)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			ext := strings.ToLower(filepath.Ext(entry.url))
			if ext == ".mp4" {
				contentType = "video/mp4"
			} else if ext == ".webm" {
				contentType = "video/webm"
			} else {
				contentType = "video/mp4"
			}
		}
		w.Header().Set("Content-Type", contentType)

		for _, h := range []string{"Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)

		flusher, canFlush := w.(http.Flusher)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return
				}
				if canFlush {
					flusher.Flush()
				}
			}
			if readErr != nil {
				return
			}
		}
	})

	a.mediaServer = &http.Server{Handler: mux}
	go a.mediaServer.Serve(ln) //nolint:errcheck
	return nil
}

// GetMediaServerURL returns the base URL of the local media server.
// Frontend builds video URLs as GetMediaServerURL() + "/media/" + serveName.
func (a *App) GetMediaServerURL() string {
	return "http://" + a.mediaServerAddr
}

func isPresignedS3URL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Query().Get("X-Amz-Signature") != "" || u.Query().Get("X-Amz-Credential") != ""
}

func (a *App) shutdown(ctx context.Context) {
	if a.mediaServer != nil {
		a.mediaServer.Close()
	}
	if a.mediaDir != "" {
		os.RemoveAll(a.mediaDir)
	}
}

func (a *App) GetConfig() config.Config {
	return *a.config
}

func (a *App) IsDevMode() bool {
	return a.config.DevMode
}

func (a *App) UpdateAPIURL(url string) error {
	a.config.APIURL = url
	return config.Save(a.config)
}

// -------------------- Auth (bound to frontend) --------------------

// apiBase returns the normalized server base URL (no trailing slash).
func (a *App) apiBase() string {
	if a.config != nil && a.config.APIURL != "" {
		return strings.TrimRight(a.config.APIURL, "/")
	}
	return "http://127.0.0.1:8080"
}

// authToken returns the stored device token for the current server, or "".
// Safe to call when no token exists — callers should proceed without auth
// and let the server's 401 surface naturally.
func (a *App) authToken() string {
	if a.config == nil {
		return ""
	}
	tok, err := clientauth.LoadDeviceToken(a.config.APIURL)
	if err != nil {
		return ""
	}
	return tok
}

// AuthStatus is what the frontend uses on launch to decide which screen to show.
type AuthStatus struct {
	ServerURL       string `json:"server_url"`
	AccountUsername string `json:"account_username,omitempty"`
	HasToken        bool   `json:"has_token"`
	NeedsSetup      bool   `json:"needs_setup"`
	Reachable       bool   `json:"reachable"`
}

func (a *App) GetAuthStatus() (*AuthStatus, error) {
	status := &AuthStatus{
		ServerURL:       a.apiBase(),
		AccountUsername: a.config.AccountUsername,
		HasToken:        a.authToken() != "",
	}
	// Probe the server's setup state. If unreachable, leave reachable=false so
	// the frontend can still show login with a "can't reach server" hint.
	req, err := http.NewRequest("GET", a.apiBase()+"/api/v1/auth/status", nil)
	if err != nil {
		return status, nil
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return status, nil
	}
	defer resp.Body.Close()
	status.Reachable = true
	if resp.StatusCode == http.StatusOK {
		var body struct {
			NeedsSetup bool `json:"needs_setup"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
			status.NeedsSetup = body.NeedsSetup
		}
	}
	return status, nil
}

// ProbeServer checks if a given URL hosts a ClipShare server and returns its
// auth status without persisting the URL to config.
func (a *App) ProbeServer(serverURL string) (*AuthStatus, error) {
	serverURL = strings.TrimRight(serverURL, "/")
	status := &AuthStatus{ServerURL: serverURL}
	req, err := http.NewRequest("GET", serverURL+"/api/v1/auth/status", nil)
	if err != nil {
		return status, nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return status, nil
	}
	defer resp.Body.Close()
	status.Reachable = true
	if resp.StatusCode == http.StatusOK {
		var body struct {
			NeedsSetup bool `json:"needs_setup"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
			status.NeedsSetup = body.NeedsSetup
		}
	}
	return status, nil
}

// SetupAdmin runs the first-time admin bootstrap using the setup token.
// Stores the resulting JWT access token in the keyring so the admin can make
// authenticated calls (no refresh token — admin re-enters their password on
// expiry, which is fine for a self-hosted tool).
func (a *App) SetupAdmin(serverURL, setupToken, username, password string) error {
	serverURL = strings.TrimRight(serverURL, "/")
	body, _ := json.Marshal(map[string]string{
		"setup_token": setupToken, "username": username, "password": password,
	})
	resp, err := postJSON(serverURL+"/api/v1/auth/setup", body, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return readErr(resp)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	return a.saveSession(serverURL, username, out.AccessToken)
}

// LoginAdmin authenticates the admin with username+password and stores the JWT.
func (a *App) LoginAdmin(serverURL, username, password string) error {
	serverURL = strings.TrimRight(serverURL, "/")
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := postJSON(serverURL+"/api/v1/auth/login", body, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return readErr(resp)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	return a.saveSession(serverURL, username, out.AccessToken)
}

// RedeemInvite swaps an invite code for a long-lived device token.
func (a *App) RedeemInvite(serverURL, code, username string) error {
	serverURL = strings.TrimRight(serverURL, "/")
	body, _ := json.Marshal(map[string]string{
		"code": code, "username": username,
	})
	resp, err := postJSON(serverURL+"/api/v1/auth/redeem", body, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return readErr(resp)
	}
	var out struct {
		DeviceToken string `json:"device_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	return a.saveSession(serverURL, username, out.DeviceToken)
}

func (a *App) saveSession(serverURL, username, token string) error {
	a.config.APIURL = serverURL
	a.config.AccountUsername = username
	if err := config.Save(a.config); err != nil {
		return err
	}
	return clientauth.SaveDeviceToken(serverURL, token)
}

// LogoutDevice clears stored credentials and best-effort notifies the server.
func (a *App) LogoutDevice() error {
	token := a.authToken()
	if token != "" {
		req, _ := http.NewRequest("DELETE", a.apiBase()+"/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	_ = clientauth.DeleteDeviceToken(a.config.APIURL)
	a.config.AccountUsername = ""
	return config.Save(a.config)
}

// GetAuthToken exposes the device token to the frontend so the React app can
// attach it to its own axios calls.
func (a *App) GetAuthToken() string {
	return a.authToken()
}

func postJSON(url string, body []byte, bearer string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

func readErr(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var j struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(b, &j); err == nil && j.Error != "" {
		return fmt.Errorf("%s", j.Error)
	}
	return fmt.Errorf("request failed: %s", strings.TrimSpace(string(b)))
}

// HandleAuthCallback is legacy magic-link deep-link handling; kept so any
// registered clipshare:// handler doesn't break the app.
func (a *App) HandleAuthCallback(rawURL string) (map[string]interface{}, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	token := parsedURL.Query().Get("token")
	if token == "" {
		return nil, fmt.Errorf("no token in URL")
	}
	return map[string]interface{}{"token": token}, nil
}

func (a *App) FFmpegIsAvailable() bool {
	return ffmpeg.IsAvailable()
}

// InstallFFmpeg downloads and installs ffmpeg to the user's app data dir.
// Progress is reported via "ffmpeg:install:progress" events:
//
//	{ "stage": "Downloading FFmpeg…", "pct": 42.0 }
func (a *App) InstallFFmpeg() error {
	return ffmpeg.Install(a.ctx, func(stage string, pct float64) {
		wailsruntime.EventsEmit(a.ctx, "ffmpeg:install:progress", map[string]interface{}{
			"stage": stage,
			"pct":   pct,
		})
	})
}

type ProbeResult struct {
	Duration   float64 `json:"duration"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
	Codec      string  `json:"codec"`
	BitrateKb  int     `json:"bitrate_kbps"`
	StreamCopy bool    `json:"stream_copy"`
}

func (a *App) ProbeVideo(inputPath string) (*ProbeResult, error) {
	result, err := ffmpeg.Probe(a.ctx, inputPath)
	if err != nil {
		return nil, err
	}
	return &ProbeResult{
		Duration:   result.Duration,
		Width:      result.Width,
		Height:     result.Height,
		FPS:        result.FPS,
		Codec:      result.Codec,
		BitrateKb:  result.Bitrate,
		StreamCopy: ffmpeg.CanStreamCopy(result, inputPath),
	}, nil
}

type TrimRequest struct {
	InputPath string  `json:"input_path"`
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration"`
}

func (a *App) ExtractThumbnail(serveName string, seekTime float64) (string, error) {
	resolved, ok := a.mediaGet(serveName)
	if !ok {
		return "", fmt.Errorf("file not found: %s", serveName)
	}

	thumbName := fmt.Sprintf("thumb-%d.jpg", time.Now().UnixNano())
	thumbPath := filepath.Join(a.mediaDir, thumbName)

	if err := ffmpeg.Thumbnail(a.ctx, ffmpeg.ThumbnailOptions{
		InputPath:  resolved,
		OutputPath: thumbPath,
		Time:       seekTime,
		Width:      640,
	}); err != nil {
		return "", fmt.Errorf("thumbnail extraction: %w", err)
	}

	a.mediaSet(thumbName, thumbPath, true) // deletable: temp file
	return thumbName, nil
}

func (a *App) TrimVideo(req TrimRequest) (string, error) {
	return a.trimVideo(req, false)
}

func (a *App) TrimVideoFast(req TrimRequest) (string, error) {
	return a.trimVideo(req, true)
}

func (a *App) trimVideo(req TrimRequest, fastPreview bool) (string, error) {
	id := mediaName("trim", atomic.AddInt64(&a.mediaSeq, 1), req.InputPath, ".mp4")
	outputPath := filepath.Join(a.mediaDir, id)

	probeResult, probeErr := ffmpeg.Probe(a.ctx, req.InputPath)

	var w, h int
	var duration float64
	if probeResult != nil {
		w = probeResult.Width
		h = probeResult.Height
		duration = probeResult.Duration
	}

	makeOpts := func(sc bool, width, height int) ffmpeg.TrimOptions {
		return ffmpeg.TrimOptions{
			InputPath:    req.InputPath,
			OutputPath:   outputPath,
			StartTime:    req.StartTime,
			Duration:     req.Duration,
			StreamCopy:   sc,
			SourceWidth:  width,
			SourceHeight: height,
		}
	}

	// For fast preview mode, try stream copy first (instant trim, full source bitrate).
	if fastPreview {
		canStreamCopy := probeErr == nil && ffmpeg.CanStreamCopy(probeResult, req.InputPath)
		if canStreamCopy {
			if err := ffmpeg.TrimWithProgress(a.ctx, makeOpts(true, w, h), duration, nil); err == nil {
				serveName := id
				a.mediaSet(serveName, outputPath, true)
				return serveName, nil
			}
			os.Remove(outputPath)
		}
		// Fallback: re-encode for preview
		err := ffmpeg.TrimWithProgress(a.ctx, makeOpts(false, w, h), duration, nil)
		if err == nil {
			serveName := id
			a.mediaSet(serveName, outputPath, true)
			return serveName, nil
		}
		os.Remove(outputPath)
		return "", err
	}

	// For final export: always re-encode to produce a web-friendly file size.
	err := ffmpeg.TrimWithProgress(a.ctx, makeOpts(false, w, h), duration, nil)
	if err == nil {
		serveName := id
		a.mediaSet(serveName, outputPath, true)
		return serveName, nil
	}
	os.Remove(outputPath)

	// Last resort: stream copy without re-encode (preserves source bitrate,
	// potentially large file, but better than failing entirely).
	if probeResult != nil {
		isH264 := probeResult.Codec == "h264" || probeResult.Codec == "avc1" || probeResult.Codec == "h264_nvenc" || probeResult.Codec == "h264_amf" || probeResult.Codec == "h264_qsv"
		ext := strings.ToLower(filepath.Ext(req.InputPath))
		isMP4 := ext == ".mp4" || ext == ".m4v" || ext == ".mov"
		if isH264 && isMP4 {
			errSc := ffmpeg.TrimWithProgress(a.ctx, makeOpts(true, 0, 0), duration, nil)
			if errSc == nil {
				serveName := id
				a.mediaSet(serveName, outputPath, true)
				return serveName, nil
			}
			os.Remove(outputPath)
		}
	}

	return "", err
}

func (a *App) OpenFileDialog() (string, error) {
	// On Windows, Wails' COM-based dialog triggers a WebView2 focus error
	// (v2.12.0 bug). Use PowerShell's Windows.Forms dialog instead — it runs
	// in its own process and has no WebView2 involvement.
	if goruntime.GOOS == "windows" {
		return openFileDialogWindows()
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select Video File",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Video Files", Pattern: "*.mp4;*.webm;*.mov;*.mkv;*.avi"},
		},
	})
}

func openFileDialogWindows() (string, error) {
	script := strings.Join([]string{
		`Add-Type -AssemblyName System.Windows.Forms`,
		`$d = New-Object System.Windows.Forms.OpenFileDialog`,
		`$d.Title = 'Select Video File'`,
		`$d.Filter = 'Video Files|*.mp4;*.webm;*.mov;*.mkv;*.avi;*.m4v|All Files|*.*'`,
		`if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { $d.FileName }`,
	}, "; ")
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		// Treat exec errors as cancellation (e.g. user closed dialog)
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *App) ServeLocalFile(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	id := mediaName("src", atomic.AddInt64(&a.mediaSeq, 1), absPath, "")
	// Store the original path directly — no copy, no symlink.
	a.mediaSet(id, absPath, false) // not deletable: user's original file
	return id, nil
}

// mediaName builds a unique, human-readable serve name from a source file path.
// Format: <prefix>-<seq>-<sanitized-basename>.<ext>
// Sanitization: keep alphanumerics, dots, hyphens, underscores; replace everything else with "_".
func mediaName(prefix string, seq int64, sourcePath, forceExt string) string {
	base := filepath.Base(sourcePath)
	ext := forceExt
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(base))
		base = base[:len(base)-len(filepath.Ext(base))]
	} else {
		base = base[:len(base)-len(filepath.Ext(base))]
	}
	var sb strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	sanitized := sb.String()
	if sanitized == "" {
		sanitized = "clip"
	}
	return fmt.Sprintf("%s-%d-%s%s", prefix, seq, sanitized, ext)
}

func (a *App) CleanupServe(name string) error {
	if e, ok := a.mediaDel(name); ok && e.deletable {
		return os.Remove(e.path)
	}
	return nil
}

func (a *App) DeleteFile(path string) error {
	return os.Remove(path)
}

// ProxyVideoURL registers a remote video URL (with optional bearer token) in the
// local media server so WebView2 can play it without CORS/auth issues.
// Returns the local proxy URL to use as the <video src>.
func (a *App) ProxyVideoURL(videoURL string, token string) (string, error) {
	if videoURL == "" {
		return "", fmt.Errorf("videoURL is required")
	}
	key := fmt.Sprintf("stream-%d", a.streamCounter)
	a.streamCounter++
	a.streamMu.Lock()
	a.streamMap[key] = streamEntry{url: videoURL, token: token}
	a.streamMu.Unlock()
	return "http://" + a.mediaServerAddr + "/stream/" + key, nil
}

func (a *App) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *App) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

type UploadResult struct {
	ClipID    string `json:"clip_id"`
	ObjectKey string `json:"object_key"`
	FileSize  int64  `json:"file_size_bytes"`
	FileName  string `json:"file_name"`
}

func (a *App) UploadFile(serveName string) (*UploadResult, error) {
	resolved, ok := a.mediaGet(serveName)
	if !ok {
		return nil, fmt.Errorf("file not found: %s", serveName)
	}

	fileInfo, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot stat file: %w", err)
	}

	apiBase := a.config.APIURL
	if apiBase == "" {
		apiBase = "http://127.0.0.1:8080"
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		part, err := writer.CreateFormFile("file", filepath.Base(resolved))
		if err != nil {
			pw.CloseWithError(fmt.Errorf("create form file: %w", err))
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			pw.CloseWithError(fmt.Errorf("copy file to form: %w", err))
			return
		}
		writer.Close()
	}()

	uploadURL := apiBase + "/api/v1/clips/upload"
	req, err := http.NewRequest("POST", uploadURL, pr)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if tok := a.authToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var uploadResp struct {
		Clip struct {
			ID        string `json:"id"`
			ObjectKey string `json:"rustfs_object_key"`
		} `json:"clip"`
		ObjectKey string `json:"object_key"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if ffmpeg.IsAvailable() {
		thumbPath := filepath.Join(a.mediaDir, fmt.Sprintf("thumb-%s.jpg", uploadResp.Clip.ID))
		if err := ffmpeg.Thumbnail(a.ctx, ffmpeg.ThumbnailOptions{
			InputPath:  resolved,
			OutputPath: thumbPath,
			Time:       0,
			Width:      640,
		}); err == nil {
			_ = a.uploadThumbnailDirect(thumbPath, uploadResp.Clip.ID, apiBase)
			os.Remove(thumbPath)
		}
	}

	return &UploadResult{
		ClipID:    uploadResp.Clip.ID,
		ObjectKey: uploadResp.ObjectKey,
		FileSize:  fileInfo.Size(),
		FileName:  filepath.Base(resolved),
	}, nil
}

func (a *App) uploadThumbnailDirect(thumbPath, clipID, apiBase string) error {
	data, err := os.ReadFile(thumbPath)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("thumbnail", "thumbnail.jpg")
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", apiBase+"/api/v1/clips/"+clipID+"/thumbnail", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if tok := a.authToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("thumbnail upload failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "ClipShare",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
