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
	goruntime "runtime"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	streamMu        sync.RWMutex
	streamMap       map[string]streamEntry
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

func (a *App) mediaLen() int {
	a.mediaMu.RLock()
	n := len(a.mediaMap)
	a.mediaMu.RUnlock()
	return n
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
		key := strings.TrimPrefix(r.URL.Path, "/stream/")

		a.streamMu.RLock()
		entry, ok := a.streamMap[key]
		a.streamMu.RUnlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		upstream, err := http.NewRequestWithContext(r.Context(), r.Method, entry.url, nil)
		if err != nil {
			http.Error(w, "bad upstream url", http.StatusInternalServerError)
			return
		}
		if entry.token != "" {
			upstream.Header.Set("Authorization", "Bearer "+entry.token)
		}
		if rng := r.Header.Get("Range"); rng != "" {
			upstream.Header.Set("Range", rng)
		}

		resp, err := http.DefaultClient.Do(upstream)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
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

func (a *App) HandleAuthCallback(rawURL string) (map[string]interface{}, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	query := parsedURL.Query()
	token := query.Get("token")
	if token == "" {
		return nil, fmt.Errorf("no token in URL")
	}

	return map[string]interface{}{
		"token": token,
	}, nil
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
	Duration  float64 `json:"duration"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FPS       float64 `json:"fps"`
	Codec     string  `json:"codec"`
	BitrateKb int     `json:"bitrate_kbps"`
}

func (a *App) ProbeVideo(inputPath string) (*ProbeResult, error) {
	result, err := ffmpeg.Probe(a.ctx, inputPath)
	if err != nil {
		return nil, err
	}
	return &ProbeResult{
		Duration:  result.Duration,
		Width:     result.Width,
		Height:    result.Height,
		FPS:       result.FPS,
		Codec:     result.Codec,
		BitrateKb: result.Bitrate,
	}, nil
}

type TrimRequest struct {
	InputPath string  `json:"input_path"`
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration"`
}

func (a *App) TrimVideo(req TrimRequest) (string, error) {
	id := fmt.Sprintf("trim-%d", a.mediaLen())
	outputPath := filepath.Join(a.mediaDir, id+".mp4")

	err := ffmpeg.TrimWithProgress(a.ctx, ffmpeg.TrimOptions{
		InputPath:  req.InputPath,
		OutputPath: outputPath,
		StartTime:  req.StartTime,
		Duration:   req.Duration,
	}, nil)
	if err != nil {
		return "", err
	}

	serveName := id + ".mp4"
	a.mediaSet(serveName, outputPath, true) // deletable: trimmed output
	return serveName, nil
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
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
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

	ext := strings.ToLower(filepath.Ext(absPath))
	id := fmt.Sprintf("src-%d%s", a.mediaLen(), ext)
	// Store the original path directly — no copy, no symlink.
	a.mediaSet(id, absPath, false) // not deletable: user's original file
	return id, nil
}

func (a *App) CleanupServe(name string) error {
	if e, ok := a.mediaDel(name); ok && e.deletable {
		return os.Remove(e.path)
	}
	return nil
}

// ProxyVideoURL registers a remote video URL (with optional bearer token) in the
// local media server so WebView2 can play it without CORS/auth issues.
// Returns the local proxy URL to use as the <video src>.
func (a *App) ProxyVideoURL(videoURL string, token string) (string, error) {
	if videoURL == "" {
		return "", fmt.Errorf("videoURL is required")
	}
	key := fmt.Sprintf("stream-%d", func() int {
		a.streamMu.RLock()
		n := len(a.streamMap)
		a.streamMu.RUnlock()
		return n
	}())
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
		apiBase = "http://localhost:8080"
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(resolved))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file to form: %w", err)
	}
	writer.Close()

	uploadURL := apiBase + "/api/v1/clips/upload"
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
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

	return &UploadResult{
		ClipID:    uploadResp.Clip.ID,
		ObjectKey: uploadResp.ObjectKey,
		FileSize:  fileInfo.Size(),
		FileName:  filepath.Base(resolved),
	}, nil
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
