package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sorag/video_stream/pkg/models"
	"github.com/sorag/video_stream/pkg/storage"
)

const (
	// HLSセグメントの長さ（秒）
	hlsSegmentTime = 6
	// 出力解像度（720p）
	outputScale = "1280:720"
	// ビデオビットレート
	videoBitrate = "2500k"
	// オーディオビットレート
	audioBitrate = "128k"
)

// Transcoder はFFmpegを使ってHLS変換を行う
type Transcoder struct {
	storage *storage.Client
}

func NewTranscoder(s *storage.Client) *Transcoder {
	return &Transcoder{storage: s}
}

// Transcode はオリジナル動画をHLSに変換してMinIOに保存する
// 返り値はMinIO上のHLSマスタープレイリストのキー（例: {videoID}/hls/index.m3u8）
func (t *Transcoder) Transcode(ctx context.Context, job *models.TranscodeJob) (string, error) {
	// 作業用の一時ディレクトリを作成
	workDir, err := os.MkdirTemp("", "transcode-"+job.VideoID+"-*")
	if err != nil {
		return "", fmt.Errorf("create work dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			slog.Warn("failed to clean up work dir", "dir", workDir, "err", err)
		}
	}()

	// MinIOからオリジナルファイルをダウンロード
	ext := filepath.Ext(job.OriginalKey)
	if ext == "" {
		ext = ".mp4"
	}
	inputPath := filepath.Join(workDir, "input"+ext)
	slog.Info("downloading original file", "key", job.OriginalKey)

	if err := t.storage.FGetObject(ctx, storage.BucketOriginals, job.OriginalKey, inputPath); err != nil {
		return "", fmt.Errorf("download original: %w", err)
	}

	// HLS出力ディレクトリを作成
	hlsDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		return "", fmt.Errorf("create hls dir: %w", err)
	}

	playlistPath := filepath.Join(hlsDir, "index.m3u8")
	segmentPattern := filepath.Join(hlsDir, "segment%03d.ts")

	// FFmpegでHLS変換
	slog.Info("starting ffmpeg transcode", "videoID", job.VideoID)
	if err := t.runFFmpeg(ctx, inputPath, playlistPath, segmentPattern); err != nil {
		return "", fmt.Errorf("ffmpeg: %w", err)
	}

	// HLSファイルをMinIOにアップロード
	hlsBaseKey := fmt.Sprintf("%s/hls", job.VideoID)
	slog.Info("uploading HLS files to minio", "baseKey", hlsBaseKey)

	if err := t.uploadHLSFiles(ctx, hlsDir, hlsBaseKey); err != nil {
		return "", fmt.Errorf("upload hls: %w", err)
	}

	masterKey := fmt.Sprintf("%s/index.m3u8", hlsBaseKey)
	return masterKey, nil
}

// runFFmpeg はFFmpegコマンドを実行する
func (t *Transcoder) runFFmpeg(ctx context.Context, inputPath, playlistPath, segmentPattern string) error {
	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",   // H.264エンコード
		"-c:a", "aac",       // AACオーディオ
		"-vf", fmt.Sprintf("scale=%s:force_original_aspect_ratio=decrease,pad=%s:-1:-1:color=black", outputScale, outputScale),
		"-b:v", videoBitrate,
		"-b:a", audioBitrate,
		"-hls_time", fmt.Sprintf("%d", hlsSegmentTime),
		"-hls_playlist_type", "vod",        // VOD用（全セグメントをプレイリストに含める）
		"-hls_segment_filename", segmentPattern,
		"-hls_flags", "independent_segments", // 各セグメントを独立して再生可能にする
		"-f", "hls",
		playlistPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg exit: %w", err)
	}
	return nil
}

// uploadHLSFiles はHLSディレクトリ内の全ファイルをMinIOにアップロードする
func (t *Transcoder) uploadHLSFiles(ctx context.Context, hlsDir, baseKey string) error {
	entries, err := os.ReadDir(hlsDir)
	if err != nil {
		return fmt.Errorf("read hls dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		localPath := filepath.Join(hlsDir, entry.Name())
		objectKey := fmt.Sprintf("%s/%s", baseKey, entry.Name())
		contentType := contentTypeForExt(filepath.Ext(entry.Name()))

		slog.Debug("uploading hls file", "key", objectKey)
		if err := t.storage.FPutObject(ctx, storage.BucketHLS, objectKey, localPath, contentType); err != nil {
			return fmt.Errorf("upload %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// contentTypeForExt は拡張子に対応するContent-Typeを返す
func contentTypeForExt(ext string) string {
	switch ext {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/MP2T"
	default:
		return "application/octet-stream"
	}
}
