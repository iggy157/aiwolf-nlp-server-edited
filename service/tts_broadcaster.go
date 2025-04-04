package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/gin-gonic/gin"
	"github.com/grafov/m3u8"
)

type TTSBroadcaster struct {
	config          model.Config
	client          *http.Client
	streamingMu     sync.Mutex
	isStreaming     bool
	lastSegmentTime time.Time
	segmentsMu      sync.RWMutex
	segmentCounter  int
	playlistMu      sync.Mutex
	playlist        *m3u8.MediaPlaylist
}

type Request struct {
	Text    string `json:"text"`
	Speaker int    `json:"speaker"`
}

const (
	SILENCE_TEMPLATE_FILE = "silence.ts"
)

func NewTTSBroadcaster(config model.Config) *TTSBroadcaster {
	return &TTSBroadcaster{
		config:          config,
		lastSegmentTime: time.Now(),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (t *TTSBroadcaster) Start() {
	if _, err := os.Stat(t.config.TTSBroadcaster.SegmentDir); os.IsNotExist(err) {
		os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755)
	}
	t.cleanupSegments()

	var err error
	t.playlist, err = m3u8.NewMediaPlaylist(math.MaxInt16, math.MaxInt16)
	if err != nil {
		slog.Error("プレイリストの作成に失敗しました", "error", err)
		return
	}
	t.playlist.TargetDuration = float64(t.config.TTSBroadcaster.TargetDuration.Seconds())
	t.playlist.SetVersion(3)
	t.playlist.Closed = false

	if err := t.buildSilenceTemplate(); err != nil {
		slog.Error("無音テンプレートの作成に失敗しました", "error", err)
		return
	}

	for range t.config.TTSBroadcaster.MinBufferSegments {
		t.addSilenceSegment()
	}
	go t.stream()
}

func (t *TTSBroadcaster) cleanupSegments() {
	files, err := os.ReadDir(t.config.TTSBroadcaster.SegmentDir)
	if err != nil {
		slog.Error("セグメントディレクトリの読み取りに失敗しました", "error", err)
		return
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".ts") || strings.HasSuffix(file.Name(), ".m3u8") {
			os.Remove(filepath.Join(t.config.TTSBroadcaster.SegmentDir, file.Name()))
		}
	}
}

func (t *TTSBroadcaster) buildSilenceTemplate() error {
	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)
	cmd := exec.Command(
		"ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=r=44100:cl=stereo:d=%f", t.config.TTSBroadcaster.TargetDuration.Seconds()),
		"-c:a", "aac",
		"-b:a", "64k",
		"-ar", "44100",
		"-ac", "2",
		"-mpegts_flags", "initial_discontinuity",
		"-mpegts_copyts", "1",
		"-f", "mpegts",
		silenceTemplatePath,
	)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func (t *TTSBroadcaster) addSilenceSegment() {
	t.segmentsMu.Lock()
	silenceSegmentName := fmt.Sprintf("segment_%d.ts", t.segmentCounter)
	t.segmentCounter++
	t.segmentsMu.Unlock()

	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)
	silenceSegmentPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, silenceSegmentName)

	if err := t.copyFile(silenceTemplatePath, silenceSegmentPath); err != nil {
		slog.Error("無音セグメントのコピーに失敗しました", "error", err)
		return
	}

	t.playlistMu.Lock()
	defer t.playlistMu.Unlock()

	segmentURI := fmt.Sprintf("segment/%s", silenceSegmentName)
	if err := t.playlist.AppendSegment(&m3u8.MediaSegment{
		URI:      segmentURI,
		Duration: t.config.TTSBroadcaster.TargetDuration.Seconds(),
	}); err != nil {
		slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err)
	}

	t.writePlaylist()
}

func (t *TTSBroadcaster) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (t *TTSBroadcaster) writePlaylist() {
	playlistPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, "playlist.m3u8")
	if err := os.WriteFile(playlistPath, t.playlist.Encode().Bytes(), 0644); err != nil {
		slog.Error("プレイリストの書き込みに失敗しました", "error", err)
	}
}

func (t *TTSBroadcaster) convertWavToSegment(data []byte, baseName string) ([]string, error) {
	tempWavFile, err := os.CreateTemp(t.config.TTSBroadcaster.SegmentDir, "temp-*.wav")
	if err != nil {
		return nil, err
	}
	tempWavPath := tempWavFile.Name()
	defer os.Remove(tempWavPath)

	if _, err := tempWavFile.Write(data); err != nil {
		tempWavFile.Close()
		return nil, err
	}
	tempWavFile.Close()

	duration, err := t.getWavDuration(tempWavPath)
	if err != nil {
		return nil, err
	}

	if duration <= t.config.TTSBroadcaster.TargetDuration.Seconds() {
		outputPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, baseName)
		cmd := exec.Command(
			"ffmpeg",
			"-i", tempWavPath,
			"-c:a", "aac",
			"-b:a", "64k",
			"-ar", "44100",
			"-ac", "2",
			"-mpegts_flags", "initial_discontinuity",
			"-mpegts_copyts", "1",
			"-f", "mpegts",
			outputPath,
		)
		if _, err := cmd.CombinedOutput(); err != nil {
			return nil, err
		}
		return []string{baseName}, nil
	}

	return t.splitWavIntoSegments(tempWavPath, baseName, duration)
}

func (t *TTSBroadcaster) getWavDuration(wavPath string) (float64, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		wavPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var duration float64
	fmt.Sscanf(string(output), "%f", &duration)
	return duration, nil
}

func (t *TTSBroadcaster) splitWavIntoSegments(wavPath, baseName string, totalDuration float64) ([]string, error) {
	baseNameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	segmentNames := []string{}

	// 一時的なディレクトリを作成
	tempDir, err := os.MkdirTemp("", "tts-split")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// まず一時的なAAC形式に変換
	tempAacPath := filepath.Join(tempDir, "temp.aac")
	convertCmd := exec.Command(
		"ffmpeg",
		"-i", wavPath,
		"-c:a", "aac",
		"-b:a", "64k",
		"-ar", "44100",
		"-ac", "2",
		tempAacPath,
	)

	if _, err := convertCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to convert to aac: %w", err)
	}

	// セグメントの数を計算
	segmentCount := int(math.Ceil(totalDuration / t.config.TTSBroadcaster.TargetDuration.Seconds()))
	segmentDuration := t.config.TTSBroadcaster.TargetDuration.Seconds()

	// 各セグメントを個別に作成
	for i := 0; i < segmentCount; i++ {
		segmentName := fmt.Sprintf("%s_part%03d.ts", baseNameWithoutExt, i)
		segmentPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, segmentName)

		startTime := float64(i) * segmentDuration

		// 最後のセグメントは残りの時間すべて
		duration := segmentDuration
		if i == segmentCount-1 {
			duration = totalDuration - startTime
		}

		segmentCmd := exec.Command(
			"ffmpeg",
			"-i", tempAacPath,
			"-ss", fmt.Sprintf("%f", startTime),
			"-t", fmt.Sprintf("%f", duration),
			"-c:a", "copy",
			"-mpegts_flags", "initial_discontinuity",
			"-mpegts_copyts", "1",
			"-f", "mpegts",
			segmentPath,
		)

		if _, err := segmentCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to create segment %d: %w", i, err)
		}

		segmentNames = append(segmentNames, segmentName)
	}

	return segmentNames, nil
}

func (t *TTSBroadcaster) stream() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		t.streamingMu.Lock()
		isStreaming := t.isStreaming
		lastTime := t.lastSegmentTime
		t.streamingMu.Unlock()

		if !isStreaming {
			elapsed := time.Since(lastTime).Seconds()

			t.playlistMu.Lock()
			segmentCount := t.playlist.Count()
			t.playlistMu.Unlock()

			if elapsed >= t.config.TTSBroadcaster.TargetDuration.Seconds()*0.8 || segmentCount < uint(t.config.TTSBroadcaster.MinBufferSegments) {
				t.addSilenceSegment()
				t.streamingMu.Lock()
				t.lastSegmentTime = time.Now()
				t.streamingMu.Unlock()
			}
		}
	}
}

func (t *TTSBroadcaster) ensureMinimumBuffer() {
	t.playlistMu.Lock()
	currentCount := t.playlist.Count()
	t.playlistMu.Unlock()

	neededSegments := t.config.TTSBroadcaster.MinBufferSegments - int(currentCount)
	if neededSegments > 0 {
		for range neededSegments {
			t.addSilenceSegment()
		}
	}
}

func (t *TTSBroadcaster) getDuration(segmentPath string) float64 {
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		return t.config.TTSBroadcaster.TargetDuration.Seconds()
	}

	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		segmentPath,
	)
	output, err := cmd.Output()
	if err != nil {
		slog.Error("セグメントの長さの取得に失敗しました", "error", err)
		return t.config.TTSBroadcaster.TargetDuration.Seconds()
	}

	var duration float64
	fmt.Sscanf(string(output), "%f", &duration)
	if duration <= 0 {
		return t.config.TTSBroadcaster.TargetDuration.Seconds()
	}

	return duration
}

func (t *TTSBroadcaster) HandlePlaylist(c *gin.Context) {
	playlistPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, "playlist.m3u8")
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(playlistPath)
}

func (t *TTSBroadcaster) HandleSegment(c *gin.Context) {
	segment := c.Param("segment")
	if segment == "" || segment == "/" {
		c.Status(http.StatusNotFound)
		return
	}

	segmentName := strings.TrimPrefix(segment, "/")
	if !strings.HasSuffix(segmentName, ".ts") || strings.Contains(segmentName, "/") || strings.Contains(segmentName, "\\") {
		c.Status(http.StatusNotFound)
		return
	}

	segmentPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, segmentName)
	cleanPath := filepath.Clean(segmentPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(t.config.TTSBroadcaster.SegmentDir)) {
		c.Status(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		slog.Error("セグメントファイルが見つかりません", "path", cleanPath)
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "video/MP2T")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(segmentPath)
}

func (t *TTSBroadcaster) HandleText(c *gin.Context) {
	var request Request
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if request.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Text is required"})
		return
	}

	t.streamingMu.Lock()
	t.isStreaming = true
	t.streamingMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		audioQueryCh := t.fetchAudioQueryAsync(ctx, request.Text, request.Speaker)

		if err := t.processTextToSpeech(ctx, audioQueryCh, request.Speaker); err != nil {
			slog.Error("音声合成に失敗しました", "error", err)
		}

		t.streamingMu.Lock()
		t.isStreaming = false
		t.lastSegmentTime = time.Now()
		t.streamingMu.Unlock()

		t.ensureMinimumBuffer()
	}()
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (t *TTSBroadcaster) fetchAudioQueryAsync(ctx context.Context, text string, speaker int) <-chan []byte {
	resultCh := make(chan []byte, 1)
	go func() {
		defer close(resultCh)

		queryURL := fmt.Sprintf("%s/audio_query?speaker=%d&text=%s",
			t.config.TTSBroadcaster.Host,
			speaker,
			url.QueryEscape(text))

		req, err := http.NewRequestWithContext(ctx, "POST", queryURL, nil)
		if err != nil {
			slog.Error("オーディオクエリリクエスト作成に失敗しました", "error", err)
			return
		}

		resp, err := t.client.Do(req)
		if err != nil {
			slog.Error("オーディオクエリ送信に失敗しました", "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("オーディオクエリに失敗しました", "status", resp.StatusCode)
			return
		}

		queryParams, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("オーディオクエリ読み取りに失敗しました", "error", err)
			return
		}

		resultCh <- queryParams
	}()
	return resultCh
}

func (t *TTSBroadcaster) processTextToSpeech(ctx context.Context, audioQueryCh <-chan []byte, speaker int) error {
	var queryParams []byte
	select {
	case <-ctx.Done():
		return ctx.Err()
	case params, ok := <-audioQueryCh:
		if !ok || params == nil {
			return fmt.Errorf("オーディオクエリの取得に失敗しました")
		}
		queryParams = params
	}

	synthURL := fmt.Sprintf("%s/synthesis?speaker=%d",
		t.config.TTSBroadcaster.Host, speaker)
	req, err := http.NewRequestWithContext(ctx, "POST", synthURL, bytes.NewBuffer(queryParams))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("合成クエリに失敗しました: %d", resp.StatusCode)
	}

	wavData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	t.segmentsMu.Lock()
	baseSegmentName := fmt.Sprintf("segment_%d.ts", t.segmentCounter)
	t.segmentCounter++
	t.segmentsMu.Unlock()

	segmentNames, err := t.convertWavToSegment(wavData, baseSegmentName)
	if err != nil {
		return fmt.Errorf("音声ファイルの変換に失敗しました: %w", err)
	}

	t.addSegmentsToPlaylist(segmentNames)
	return nil
}

func (t *TTSBroadcaster) addSegmentsToPlaylist(segmentNames []string) {
	if len(segmentNames) == 0 {
		return
	}

	t.playlistMu.Lock()
	defer t.playlistMu.Unlock()

	for _, segmentName := range segmentNames {
		segmentURI := fmt.Sprintf("segment/%s", segmentName)
		duration := t.getDuration(filepath.Join(t.config.TTSBroadcaster.SegmentDir, segmentName))

		if err := t.playlist.AppendSegment(&m3u8.MediaSegment{
			URI:      segmentURI,
			Duration: duration,
		}); err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err)
		}
	}

	t.writePlaylist()
}
