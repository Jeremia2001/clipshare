package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	binMu    sync.RWMutex
	binCache string // non-empty once found
)

type encoderConfig struct {
	name         string
	args         []string
	audioBitrate string
}

var hwEncoders = []encoderConfig{
	{"h264_amf", []string{"-quality", "quality", "-rc", "vbr_peak", "-vbaq", "1", "-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1"}, "160k"},
	{"h264_nvenc", []string{"-preset", "p5", "-tune", "hq", "-rc", "vbr", "-cq", "22", "-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1", "-spatial-aq", "1", "-temporal-aq", "1"}, "160k"},
	{"h264_qsv", []string{"-preset", "slow", "-global_quality", "22", "-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1"}, "160k"},
}

var swEncoder = encoderConfig{
	"libx264", []string{"-preset", "medium", "-crf", "22", "-pix_fmt", "yuv420p", "-profile:v", "high", "-level", "4.1"}, "160k",
}

type bitrateParams struct {
	maxrate string
	bufsize string
	bv      string
}

func resolutionBitrate(w, h int) bitrateParams {
	pixels := w * h
	switch {
	case pixels <= 1280*720:
		return bitrateParams{maxrate: "6M", bufsize: "12M", bv: "4M"}
	case pixels <= 1920*1080:
		return bitrateParams{maxrate: "10M", bufsize: "20M", bv: "7M"}
	case pixels <= 2560*1440:
		return bitrateParams{maxrate: "16M", bufsize: "32M", bv: "11M"}
	default:
		return bitrateParams{maxrate: "24M", bufsize: "48M", bv: "16M"}
	}
}

var (
	encMu     sync.Mutex
	encCached *encoderConfig
)

// selectEncoder detects the best available H.264 encoder once and caches it.
func selectEncoder() encoderConfig {
	encMu.Lock()
	defer encMu.Unlock()
	if encCached != nil {
		return *encCached
	}
	bin, err := ffmpegBin()
	if err != nil {
		encCached = &swEncoder
		return swEncoder
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, enc := range hwEncoders {
		// Include -pix_fmt yuv420p in the probe so we test the encoder with
		// the exact pixel format we'll use during actual encoding.
		args := []string{"-f", "lavfi", "-i", "nullsrc=s=320x240:d=0.04", "-frames:v", "1", "-c:v", enc.name, "-pix_fmt", "yuv420p", "-f", "null", "-"}
		cmd := exec.CommandContext(ctx, bin, args...)
		if cmd.Run() == nil {
			e := enc
			encCached = &e
			return e
		}
	}
	encCached = &swEncoder
	return swEncoder
}

func invalidateBinCache() {
	binMu.Lock()
	binCache = ""
	binMu.Unlock()
}

func ffmpegExe() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func ffprobeExe() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}
	return "ffprobe"
}

func findBin() (string, error) {
	binMu.RLock()
	cached := binCache
	binMu.RUnlock()
	if cached != "" {
		return cached, nil
	}

	binMu.Lock()
	defer binMu.Unlock()

	// Double-checked locking.
	if binCache != "" {
		return binCache, nil
	}

	setBin := func(dir string) string {
		binCache = dir
		return dir
	}

	// 1. App data install dir (populated by Install()).
	if installDir, err := InstallDir(); err == nil {
		if hasBinaries(installDir) {
			return setBin(installDir), nil
		}
	}

	// 2. Alongside the executable.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if hasBinaries(dir) {
			return setBin(dir), nil
		}
	}

	// 3. System PATH.
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found in PATH or app data dir")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return "", fmt.Errorf("ffprobe not found in PATH")
	}
	return setBin(filepath.Dir(ffmpegPath)), nil
}

func hasBinaries(dir string) bool {
	_, errF := os.Stat(filepath.Join(dir, ffmpegExe()))
	_, errP := os.Stat(filepath.Join(dir, ffprobeExe()))
	return errF == nil && errP == nil
}

func ffmpegBin() (string, error) {
	dir, err := findBin()
	if err != nil {
		return "", err
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	return filepath.Join(dir, name), nil
}

func ffprobeBin() (string, error) {
	dir, err := findBin()
	if err != nil {
		return "", err
	}
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	return filepath.Join(dir, name), nil
}

type ProbeResult struct {
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	Codec    string  `json:"codec"`
	Bitrate  int     `json:"bitrate_kbps"`
}

func Probe(ctx context.Context, inputPath string) (*ProbeResult, error) {
	bin, err := ffprobeBin()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecName  string `json:"codec_name"`
			CodecType  string `json:"codec_type"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			BitRate    string `json:"bit_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("ffprobe output parse error: %w", err)
	}

	result := &ProbeResult{}

	if dur, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		result.Duration = dur
	}
	if br, err := strconv.Atoi(probe.Format.BitRate); err == nil {
		result.Bitrate = br / 1000
	}

	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			result.Width = s.Width
			result.Height = s.Height
			result.Codec = s.CodecName
			if br, err := strconv.Atoi(s.BitRate); err == nil && br > 0 {
				result.Bitrate = br / 1000
			}
			if s.RFrameRate != "" {
				parts := strings.Split(s.RFrameRate, "/")
				if len(parts) == 2 {
					num, err1 := strconv.ParseFloat(parts[0], 64)
					den, err2 := strconv.ParseFloat(parts[1], 64)
					if err1 == nil && err2 == nil && den > 0 {
						result.FPS = num / den
					}
				}
			}
			break
		}
	}

	return result, nil
}

type TrimOptions struct {
	InputPath    string  `json:"input_path"`
	OutputPath   string  `json:"output_path"`
	StartTime    float64 `json:"start_time"`
	Duration     float64 `json:"duration"`
	StreamCopy   bool    `json:"stream_copy"`
	SourceWidth  int     `json:"source_width"`
	SourceHeight int     `json:"source_height"`
}

func needsDownscale(w, h int) bool {
	return w > 1920 || h > 1080
}

func scaleFilter(h int) string {
	if h > 1080 {
		return "scale=-2:1080,format=yuv420p"
	}
	return "scale=1920:-2,format=yuv420p"
}

func trimArgs(opts TrimOptions, enc encoderConfig, progress bool) []string {
	if opts.StreamCopy {
		args := []string{
			"-ss", fmt.Sprintf("%.3f", opts.StartTime),
			"-i", opts.InputPath,
			"-t", fmt.Sprintf("%.3f", opts.Duration),
			"-c:v", "copy",
			"-c:a", "copy",
			"-movflags", "+faststart",
			"-y", opts.OutputPath,
		}
		return args
	}

	args := []string{
		"-ss", fmt.Sprintf("%.3f", opts.StartTime),
		"-noaccurate_seek",
		"-i", opts.InputPath,
		"-t", fmt.Sprintf("%.3f", opts.Duration),
		"-c:v", enc.name,
	}

	if needsDownscale(opts.SourceWidth, opts.SourceHeight) {
		args = append(args, "-vf", scaleFilter(opts.SourceHeight))
	}

	args = append(args, enc.args...)

	bp := resolutionBitrate(opts.SourceWidth, opts.SourceHeight)
	args = append(args, "-maxrate", bp.maxrate, "-bufsize", bp.bufsize, "-b:v", bp.bv)

	args = append(args, "-c:a", "aac", "-b:a", enc.audioBitrate, "-movflags", "+faststart")
	if progress {
		args = append(args, "-progress", "pipe:1")
	}
	args = append(args, "-y", opts.OutputPath)
	return args
}

func Trim(ctx context.Context, opts TrimOptions) error {
	bin, err := ffmpegBin()
	if err != nil {
		return err
	}
	if opts.OutputPath == "" {
		return fmt.Errorf("output_path is required")
	}
	enc := selectEncoder()
	err = trimEncode(ctx, bin, opts, enc)
	if err != nil && enc.name != swEncoder.name {
		err = trimEncode(ctx, bin, opts, swEncoder)
	}
	return err
}

func trimEncode(ctx context.Context, bin string, opts TrimOptions, enc encoderConfig) error {
	args := trimArgs(opts, enc, false)
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg trim failed (%s): %w\ncommand: %s %s\nstderr: %s", enc.name, err, bin, strings.Join(args, " "), stderr.String())
	}
	return nil
}

type ThumbnailOptions struct {
	InputPath  string  `json:"input_path"`
	OutputPath string  `json:"output_path"`
	Time       float64 `json:"time"`
	Width      int     `json:"width"`
}

func Thumbnail(ctx context.Context, opts ThumbnailOptions) error {
	bin, err := ffmpegBin()
	if err != nil {
		return err
	}

	args := []string{
		"-ss", fmt.Sprintf("%.3f", opts.Time),
		"-i", opts.InputPath,
		"-vframes", "1",
	}

	if opts.Width > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:-1", opts.Width))
	}

	args = append(args, "-q:v", "2", "-y", opts.OutputPath)

	return cmdRun(ctx, bin, args)
}

func cmdRun(ctx context.Context, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\n%s", err, stderr.String())
	}
	return nil
}

type ProgressCallback func(progress float64)

func TrimWithProgress(ctx context.Context, opts TrimOptions, totalDuration float64, cb ProgressCallback) error {
	if opts.StreamCopy {
		return Trim(ctx, opts)
	}

	bin, err := ffmpegBin()
	if err != nil {
		return err
	}

	if totalDuration <= 0 {
		probeResult, err := Probe(ctx, opts.InputPath)
		if err != nil {
			return Trim(ctx, opts)
		}
		totalDuration = probeResult.Duration
	}

	enc := selectEncoder()
	err = trimWithProgressEncoder(ctx, bin, opts, enc, totalDuration, cb)
	if err != nil && enc.name != swEncoder.name {
		err = trimWithProgressEncoder(ctx, bin, opts, swEncoder, totalDuration, cb)
	}
	return err
}

func trimWithProgressEncoder(ctx context.Context, bin string, opts TrimOptions, enc encoderConfig, totalDuration float64, cb ProgressCallback) error {
	args := trimArgs(opts, enc, true)

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return trimEncode(ctx, bin, opts, enc)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed (%s): %w", enc.name, err)
	}

	timeRe := regexp.MustCompile(`out_time_us=(\d+)`)
	buf := make([]byte, 4096)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 && cb != nil && totalDuration > 0 {
			chunk := string(buf[:n])
			if matches := timeRe.FindStringSubmatch(chunk); len(matches) == 2 {
				if us, parseErr := strconv.ParseInt(matches[1], 10, 64); parseErr == nil {
					processed := float64(us) / 1_000_000.0
					pct := processed / totalDuration * 100
					if pct > 100 {
						pct = 100
					}
					cb(pct)
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg trim failed (%s): %w\ncommand: %s %s\nstderr: %s", enc.name, err, bin, strings.Join(args, " "), stderr.String())
	}
	if cb != nil {
		cb(100)
	}
	return nil
}

func IsAvailable() bool {
	_, err := findBin()
	return err == nil
}

func CanStreamCopy(probeResult *ProbeResult, inputPath string) bool {
	ext := strings.ToLower(filepath.Ext(inputPath))
	isMP4Container := ext == ".mp4" || ext == ".m4v" || ext == ".mov"
	isH264 := probeResult.Codec == "h264" || probeResult.Codec == "avc1" || probeResult.Codec == "h264_nvenc" || probeResult.Codec == "h264_amf" || probeResult.Codec == "h264_qsv"
	return isMP4Container && isH264 && !needsDownscale(probeResult.Width, probeResult.Height)
}

func CanStreamCopyPath(ctx context.Context, inputPath string) bool {
	result, err := Probe(ctx, inputPath)
	if err != nil {
		return false
	}
	return CanStreamCopy(result, inputPath)
}
