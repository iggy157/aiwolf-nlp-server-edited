package service

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
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
	config            model.Config
	segmentsMu        sync.RWMutex
	playlistMu        sync.Mutex
	segmentCounter    int
	segmentDir        string
	maxSegments       int
	isStreaming       bool
	streamingMu       sync.Mutex
	targetDuration    float64
	playlist          *m3u8.MediaPlaylist
	lastSegmentTime   time.Time
	minBufferSegments int
	silenceTemplate   string // 無音テンプレートファイルのパス
}

type Request struct {
	Text    string `json:"text"`
	Speaker int    `json:"speaker"`
}

const (
	SILENCE_TEMPLATE_FILE = "silence_template.ts" // 無音テンプレートファイル
	TARGET_DURATION       = 2.0                   // 2秒に設定
	MIN_BUFFER_SEGMENTS   = 3                     // プレイヤーが追いつかないようにするための最小バッファセグメント数
)

func NewTTSBroadcaster(config model.Config) *TTSBroadcaster {
	return &TTSBroadcaster{
		config:            config,
		segmentDir:        config.TTSBroadcaster.SegmentDir,
		maxSegments:       config.TTSBroadcaster.MaxSegments,
		targetDuration:    TARGET_DURATION,
		lastSegmentTime:   time.Now(),
		minBufferSegments: MIN_BUFFER_SEGMENTS,
	}
}

func (t *TTSBroadcaster) Start() {
	if _, err := os.Stat(t.segmentDir); os.IsNotExist(err) {
		os.MkdirAll(t.segmentDir, 0755)
	}
	// 既存のセグメントファイルをクリーンアップ
	t.cleanupSegments()

	// プレイリストの初期化
	var err error
	t.playlist, err = m3u8.NewMediaPlaylist(uint(t.maxSegments), uint(t.maxSegments))
	if err != nil {
		slog.Error("プレイリストの作成に失敗しました", "error", err)
		return
	}
	t.playlist.TargetDuration = t.targetDuration
	t.playlist.SetVersion(3)
	t.playlist.Closed = false

	// 無音テンプレートファイルの作成
	if err := t.buildSilenceTemplate(); err != nil {
		slog.Error("無音テンプレートの作成に失敗しました", "error", err)
		return
	}

	// 初期セグメントとして複数の無音を追加して、プレイヤーが追いつかないようにする
	for i := 0; i < t.minBufferSegments; i++ {
		t.addSilenceSegment()
	}

	go t.stream()
}

func (t *TTSBroadcaster) cleanupSegments() {
	// セグメントディレクトリ内の古いファイルを削除
	files, err := os.ReadDir(t.segmentDir)
	if err != nil {
		slog.Error("セグメントディレクトリの読み取りに失敗しました", "error", err)
		return
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".ts") || strings.HasSuffix(file.Name(), ".m3u8") {
			os.Remove(filepath.Join(t.segmentDir, file.Name()))
		}
	}
}

// 無音テンプレートファイルを作成
func (t *TTSBroadcaster) buildSilenceTemplate() error {
	t.silenceTemplate = filepath.Join(t.segmentDir, SILENCE_TEMPLATE_FILE)
	cmd := exec.Command(
		"ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=r=44100:cl=stereo:d=%f", t.targetDuration),
		"-c:a", "aac",
		"-b:a", "128k",
		"-f", "mpegts",
		t.silenceTemplate,
	)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

// 無音セグメントを追加（segment_プレフィックスを使用）
func (t *TTSBroadcaster) addSilenceSegment() {
	// 無音テンプレートをコピーして新しいセグメントファイルを作成
	silenceSegmentName := fmt.Sprintf("segment_%d.ts", t.segmentCounter)
	t.segmentCounter++

	silenceSegmentPath := filepath.Join(t.segmentDir, silenceSegmentName)

	// テンプレートファイルをコピー
	if err := t.copyFile(t.silenceTemplate, silenceSegmentPath); err != nil {
		slog.Error("無音セグメントのコピーに失敗しました", "error", err)
		return
	}

	t.segmentsMu.Lock()
	defer t.segmentsMu.Unlock()

	// プレイリストにセグメントを追加
	t.playlistMu.Lock()
	defer t.playlistMu.Unlock()

	segmentURI := fmt.Sprintf("segment/%s", silenceSegmentName)

	if err := t.playlist.AppendSegment(&m3u8.MediaSegment{
		URI:      segmentURI,
		Duration: t.targetDuration,
	}); err != nil {
		slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err)
	}

	// プレイリストをファイルに書き込み
	t.writePlaylist()
}

// ファイルをコピーするヘルパー関数
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

// プレイリストをファイルに書き込む
func (t *TTSBroadcaster) writePlaylist() {
	playlistPath := filepath.Join(t.segmentDir, "playlist.m3u8")
	if err := os.WriteFile(playlistPath, t.playlist.Encode().Bytes(), 0644); err != nil {
		slog.Error("プレイリストの書き込みに失敗しました", "error", err)
	}
}

func (t *TTSBroadcaster) convertWavToSegment(data []byte, baseName string) ([]string, error) {
	tempWavFile, err := os.CreateTemp(t.segmentDir, "temp-*.wav")
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

	// Get duration of the wav file
	duration, err := t.getWavDuration(tempWavPath)
	if err != nil {
		return nil, err
	}

	// If duration is less than or equal to target duration, no need to split
	if duration <= t.targetDuration {
		outputPath := filepath.Join(t.segmentDir, baseName)
		cmd := exec.Command(
			"ffmpeg",
			"-i", tempWavPath,
			"-c:a", "aac",
			"-b:a", "128k",
			"-f", "mpegts",
			outputPath,
		)
		if _, err := cmd.CombinedOutput(); err != nil {
			return nil, err
		}
		return []string{baseName}, nil
	}

	// Need to split the file into segments of targetDuration
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
	segmentCount := int(totalDuration/t.targetDuration) + 1
	segmentNames := make([]string, 0, segmentCount)

	// Create a temporary directory for intermediate files
	tempDir, err := os.MkdirTemp("", "tts-split")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// Split the audio file into segments
	baseNameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Use ffmpeg segment muxer for more reliable splitting
	segmentPattern := filepath.Join(t.segmentDir, fmt.Sprintf("%s_part%%03d.ts", baseNameWithoutExt))
	cmd := exec.Command(
		"ffmpeg",
		"-i", wavPath,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%f", t.targetDuration),
		"-segment_list_type", "m3u8",
		"-c:a", "aac",
		"-b:a", "128k",
		"-map", "0:a",
		"-f", "mpegts",
		segmentPattern,
	)
	if _, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to split segments: %w", err)
	}

	// Find all generated segment files
	pattern := fmt.Sprintf("%s_part*.ts", baseNameWithoutExt)
	matches, err := filepath.Glob(filepath.Join(t.segmentDir, pattern))
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		segmentNames = append(segmentNames, filepath.Base(match))
	}

	return segmentNames, nil
}

func (t *TTSBroadcaster) stream() {
	ticker := time.NewTicker(500 * time.Millisecond) // より頻繁にチェック
	defer ticker.Stop()

	for range ticker.C {
		t.streamingMu.Lock()
		isStreaming := t.isStreaming
		lastTime := t.lastSegmentTime
		t.streamingMu.Unlock()

		if !isStreaming {
			// 最後のセグメント追加からの経過時間を計算
			elapsed := time.Since(lastTime).Seconds()

			// プレイリストのセグメント数を確認
			t.playlistMu.Lock()
			segmentCount := t.playlist.Count()
			t.playlistMu.Unlock()

			// 経過時間がtargetDuration以上、またはセグメント数が最小バッファ未満の場合に無音セグメントを追加
			if elapsed >= t.targetDuration*0.8 || segmentCount < uint(t.minBufferSegments) {
				t.addSilenceSegment()
				t.streamingMu.Lock()
				t.lastSegmentTime = time.Now()
				t.streamingMu.Unlock()
			}
		}
	}
}

// 最小バッファセグメント数を確保するための新しいメソッド
func (t *TTSBroadcaster) ensureMinimumBuffer() {
	t.playlistMu.Lock()
	currentCount := t.playlist.Count()
	t.playlistMu.Unlock()

	// 現在のセグメント数が最小バッファ未満なら無音セグメントを追加
	neededSegments := t.minBufferSegments - int(currentCount)
	if neededSegments > 0 {
		for i := 0; i < neededSegments; i++ {
			t.addSilenceSegment()
		}
	}
}

func (t *TTSBroadcaster) getDuration(segmentPath string) float64 {
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		return t.targetDuration
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
		return t.targetDuration
	}

	var duration float64
	fmt.Sscanf(string(output), "%f", &duration)
	if duration <= 0 {
		return t.targetDuration
	}

	return duration
}

func (t *TTSBroadcaster) HandlePlaylist(c *gin.Context) {
	playlistPath := filepath.Join(t.segmentDir, "playlist.m3u8")
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
	// Extract segment name from the path parameter
	segmentPath := c.Param("segment")
	if segmentPath == "" || segmentPath == "/" {
		c.Status(http.StatusNotFound)
		return
	}

	// Remove leading slash if present
	segmentName := strings.TrimPrefix(segmentPath, "/")
	fullPath := filepath.Join(t.segmentDir, segmentName)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		slog.Error("セグメントファイルが見つかりません", "path", fullPath)
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "video/MP2T")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(fullPath)
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
		if err := t.addTextSegment(request.Text, request.Speaker); err != nil {
			slog.Error("音声合成に失敗しました", "error", err)
		}

		t.streamingMu.Lock()
		t.isStreaming = false
		t.lastSegmentTime = time.Now()
		t.streamingMu.Unlock()

		// 音声セグメント追加後、バッファを確保するために無音セグメントを追加
		t.ensureMinimumBuffer()
	}()

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (t *TTSBroadcaster) addTextSegment(text string, speaker int) error {
	// Step 1: Get audio query
	queryURL := fmt.Sprintf("http://localhost:50021/audio_query?speaker=%d&text=%s",
		speaker,
		url.QueryEscape(text))
	resp, err := http.Post(queryURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("オーディオクエリに失敗しました: %d", resp.StatusCode)
	}

	queryParams, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Step 2: Synthesize audio
	synthURL := fmt.Sprintf("http://localhost:50021/synthesis?speaker=%d", speaker)
	resp, err = http.Post(synthURL, "application/json", bytes.NewBuffer(queryParams))
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

	// Step 3: Create segment file(s)
	baseSegmentName := fmt.Sprintf("segment_%d.ts", t.segmentCounter)
	t.segmentCounter++

	// Convert and potentially split the wav file into segments
	segmentNames, err := t.convertWavToSegment(wavData, baseSegmentName)
	if err != nil {
		return fmt.Errorf("音声ファイルの変換に失敗しました: %w", err)
	}

	// Step 4: Update playlist with all segments
	t.addSegmentsToPlaylist(segmentNames)

	return nil
}

// プレイリストにセグメントを追加する共通メソッド
func (t *TTSBroadcaster) addSegmentsToPlaylist(segmentNames []string) {
	t.segmentsMu.Lock()
	defer t.segmentsMu.Unlock()

	t.playlistMu.Lock()
	defer t.playlistMu.Unlock()

	for _, segmentName := range segmentNames {
		// プレイリストにセグメントを追加
		segmentURI := fmt.Sprintf("segment/%s", segmentName)
		duration := t.getDuration(filepath.Join(t.segmentDir, segmentName))

		if err := t.playlist.AppendSegment(&m3u8.MediaSegment{
			URI:      segmentURI,
			Duration: duration,
		}); err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err)
		}
	}

	// プレイリストをファイルに書き込み
	t.writePlaylist()
}
