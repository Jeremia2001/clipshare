package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"clipshare-desktop/internal/config"
	"clipshare-desktop/internal/ffmpeg"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	ctx      context.Context
	config   *config.Config
	mediaDir string
	mediaMap map[string]string
}

func NewApp() *App {
	return &App{
		mediaMap: make(map[string]string),
	}
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
}

func (a *App) shutdown(ctx context.Context) {
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
	id := fmt.Sprintf("trim-%d", len(a.mediaMap))
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
	a.mediaMap[serveName] = outputPath
	return serveName, nil
}

func (a *App) OpenFileDialog() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Video File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Video Files", Pattern: "*.mp4;*.webm;*.mov;*.mkv;*.avi"},
		},
	})
	return file, err
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
	id := fmt.Sprintf("src-%d%s", len(a.mediaMap), ext)
	linkPath := filepath.Join(a.mediaDir, id)

	if err := os.Symlink(absPath, linkPath); err != nil {
		err := copyFile(absPath, linkPath)
		if err != nil {
			return "", err
		}
	}

	serveName := id
	a.mediaMap[serveName] = linkPath
	return serveName, nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("cannot open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("cannot create copy: %w", err)
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(src); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("cannot copy file: %w", err)
	}
	return nil
}

func (a *App) CleanupServe(name string) error {
	if linkPath, ok := a.mediaMap[name]; ok {
		delete(a.mediaMap, name)
		return os.Remove(linkPath)
	}
	return nil
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
	resolved, ok := a.mediaMap[serveName]
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

func (a *App) mediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/media/")
		name = strings.TrimSuffix(name, "/")

		resolved, ok := a.mediaMap[name]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if _, err := os.Stat(resolved); err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, resolved)
	})
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
			Middleware: func(next http.Handler) http.Handler {
				mh := app.mediaHandler()
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/media/") {
						mh.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
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
