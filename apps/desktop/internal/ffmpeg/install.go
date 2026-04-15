package ffmpeg

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ulikunitz/xz"
)

// ProgressFunc is called with a human-readable stage and a percentage 0–100.
type ProgressFunc func(stage string, pct float64)

// InstallDir returns the OS-specific directory where ffmpeg is (or will be) installed.
func InstallDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		return filepath.Join(base, "ClipShare", "ffmpeg"), nil
	case "linux":
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "clipshare", "ffmpeg"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "ClipShare", "ffmpeg"), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// EnsureAvailable checks if ffmpeg is usable; if not, downloads and installs it.
func EnsureAvailable(ctx context.Context, onProgress ProgressFunc) error {
	if IsAvailable() {
		return nil
	}
	return Install(ctx, onProgress)
}

// Install downloads ffmpeg + ffprobe to the app data dir for the current OS/arch.
func Install(ctx context.Context, onProgress ProgressFunc) error {
	dir, err := InstallDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		err = installWindows(ctx, dir, onProgress)
	case "linux":
		err = installLinux(ctx, dir, onProgress)
	case "darwin":
		err = installDarwin(ctx, dir, onProgress)
	default:
		return fmt.Errorf("automatic FFmpeg install is not supported on %s; please install ffmpeg manually", runtime.GOOS)
	}
	if err != nil {
		return err
	}

	// Invalidate findBin cache so the next call picks up the new binaries.
	invalidateBinCache()
	return nil
}

// ---- Windows ---------------------------------------------------------------

func installWindows(ctx context.Context, dir string, onProgress ProgressFunc) error {
	var url string
	switch runtime.GOARCH {
	case "amd64":
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	case "arm64":
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-winarm64-gpl.zip"
	default:
		return fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
	}

	data, err := downloadWithProgress(ctx, url, onProgress, "Downloading FFmpeg…")
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	emitProgress(onProgress, "Extracting…", 0)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	return extractFromZip(zr, map[string]string{
		"ffmpeg.exe":  filepath.Join(dir, "ffmpeg.exe"),
		"ffprobe.exe": filepath.Join(dir, "ffprobe.exe"),
	})
}

// ---- Linux -----------------------------------------------------------------

func installLinux(ctx context.Context, dir string, onProgress ProgressFunc) error {
	var url string
	switch runtime.GOARCH {
	case "amd64":
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	case "arm64":
		url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz"
	default:
		return fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
	}

	data, err := downloadWithProgress(ctx, url, onProgress, "Downloading FFmpeg…")
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	emitProgress(onProgress, "Extracting…", 0)
	xzr, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open xz: %w", err)
	}
	return extractFromTar(tar.NewReader(xzr), map[string]string{
		"ffmpeg":  filepath.Join(dir, "ffmpeg"),
		"ffprobe": filepath.Join(dir, "ffprobe"),
	})
}

// ---- macOS -----------------------------------------------------------------

func installDarwin(ctx context.Context, dir string, onProgress ProgressFunc) error {
	type dl struct {
		url, name, dst string
	}
	items := []dl{
		{"https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip", "ffmpeg", filepath.Join(dir, "ffmpeg")},
		{"https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip", "ffprobe", filepath.Join(dir, "ffprobe")},
	}

	for i, item := range items {
		stage := fmt.Sprintf("Downloading %s… (%d/2)", item.name, i+1)
		data, err := downloadWithProgress(ctx, item.url, onProgress, stage)
		if err != nil {
			return fmt.Errorf("download %s: %w", item.name, err)
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("open zip for %s: %w", item.name, err)
		}
		if err := extractFromZip(zr, map[string]string{item.name: item.dst}); err != nil {
			return err
		}
	}
	return nil
}

// ---- helpers ---------------------------------------------------------------

func downloadWithProgress(ctx context.Context, url string, onProgress ProgressFunc, stage string) ([]byte, error) {
	emitProgress(onProgress, stage, 0)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	var buf bytes.Buffer
	if total > 0 {
		buf.Grow(int(total))
	}

	_, err = io.Copy(&buf, &progressReader{
		r:     resp.Body,
		total: total,
		cb: func(n int64) {
			if total > 0 {
				emitProgress(onProgress, stage, float64(n)/float64(total)*100)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type progressReader struct {
	r     io.Reader
	total int64
	read  int64
	cb    func(int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	pr.cb(pr.read)
	return n, err
}

func emitProgress(fn ProgressFunc, stage string, pct float64) {
	if fn != nil {
		fn(stage, pct)
	}
}

// extractFromZip writes files matched by base name to the mapped destination paths.
func extractFromZip(zr *zip.Reader, want map[string]string) error {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		dst, ok := want[base]
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		err = writeExecutable(dst, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("write %s: %w", base, err)
		}
		delete(want, base)
		if len(want) == 0 {
			break
		}
	}
	return checkMissing(want)
}

// extractFromTar writes files matched by base name to the mapped destination paths.
func extractFromTar(tr *tar.Reader, want map[string]string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		base := filepath.Base(hdr.Name)
		dst, ok := want[base]
		if !ok {
			continue
		}
		if err := writeExecutable(dst, tr); err != nil {
			return fmt.Errorf("write %s: %w", base, err)
		}
		delete(want, base)
		if len(want) == 0 {
			break
		}
	}
	return checkMissing(want)
}

func writeExecutable(dst string, r io.Reader) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(f, r)
	closeErr := f.Close()
	if cpErr != nil {
		os.Remove(dst)
		return cpErr
	}
	return closeErr
}

func checkMissing(want map[string]string) error {
	if len(want) == 0 {
		return nil
	}
	missing := make([]string, 0, len(want))
	for k := range want {
		missing = append(missing, k)
	}
	return fmt.Errorf("not found in archive: %s", strings.Join(missing, ", "))
}
