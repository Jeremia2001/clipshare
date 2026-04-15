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
	"strconv"
	"strings"
	"sync"
)

var (
	binPath     string
	binPathOnce sync.Once
	binPathErr  error
)

func findBin() (string, error) {
	binPathOnce.Do(func() {
		exe, _ := os.Executable()
		binDir := filepath.Dir(exe)

		for _, dir := range []string{binDir} {
			ffmpeg := filepath.Join(dir, "ffmpeg")
			ffprobe := filepath.Join(dir, "ffprobe")
			if _, err := os.Stat(ffmpeg); err == nil {
				if _, err := os.Stat(ffprobe); err == nil {
					binPath = dir
					return
				}
			}
		}

		ffmpegPath, lookErr := exec.LookPath("ffmpeg")
		if lookErr != nil {
			binPathErr = fmt.Errorf("ffmpeg not found in PATH or alongside binary: %w", lookErr)
			return
		}
		ffprobePath, lookErr2 := exec.LookPath("ffprobe")
		if lookErr2 != nil {
			binPathErr = fmt.Errorf("ffprobe not found in PATH or alongside binary: %w", lookErr2)
			return
		}
		binPath = filepath.Dir(ffmpegPath)
		if bp2 := filepath.Dir(ffprobePath); bp2 != binPath {
			if _, statErr := os.Stat(filepath.Join(binPath, "ffprobe")); statErr != nil {
				binPath = bp2
			}
		}
	})
	return binPath, binPathErr
}

func ffmpegBin() (string, error) {
	dir, err := findBin()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ffmpeg"), nil
}

func ffprobeBin() (string, error) {
	dir, err := findBin()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ffprobe"), nil
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
	InputPath  string  `json:"input_path"`
	OutputPath string  `json:"output_path"`
	StartTime  float64 `json:"start_time"`
	Duration   float64 `json:"duration"`
}

func Trim(ctx context.Context, opts TrimOptions) error {
	bin, err := ffmpegBin()
	if err != nil {
		return err
	}

	if opts.OutputPath == "" {
		return fmt.Errorf("output_path is required")
	}

	args := []string{
		"-i", opts.InputPath,
		"-ss", fmt.Sprintf("%.3f", opts.StartTime),
		"-t", fmt.Sprintf("%.3f", opts.Duration),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-movflags", "+faststart",
		"-preset", "fast",
		"-crf", "18",
		"-y",
		opts.OutputPath,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg trim failed: %w\n%s", err, stderr.String())
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
		"-i", opts.InputPath,
		"-ss", fmt.Sprintf("%.3f", opts.Time),
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

func TrimWithProgress(ctx context.Context, opts TrimOptions, cb ProgressCallback) error {
	bin, err := ffmpegBin()
	if err != nil {
		return err
	}

	probeResult, err := Probe(ctx, opts.InputPath)
	if err != nil {
		return Trim(ctx, opts)
	}
	totalDuration := probeResult.Duration

	args := []string{
		"-i", opts.InputPath,
		"-ss", fmt.Sprintf("%.3f", opts.StartTime),
		"-t", fmt.Sprintf("%.3f", opts.Duration),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-movflags", "+faststart",
		"-preset", "fast",
		"-crf", "18",
		"-progress", "pipe:1",
		"-y",
		opts.OutputPath,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Trim(ctx, opts)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
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
		return fmt.Errorf("ffmpeg trim failed: %w\n%s", err, stderr.String())
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
