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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/gin-gonic/gin"
	"github.com/grafov/m3u8"
)

type TTSBroadcaster struct {
	config    model.Config
	baseURL   *url.URL
	client    *http.Client
	streamsMu sync.RWMutex
	streams   map[string]*Stream
}

type Stream struct {
	isStreaming     bool
	lastSegmentTime time.Time
	segmentCounter  int
	playlist        *m3u8.MediaPlaylist
	streamingMu     sync.Mutex
	segmentsMu      sync.RWMutex
	playlistMu      sync.Mutex
}

const (
	SILENCE_TEMPLATE_FILE = "silence.ts"
)

func NewTTSBroadcaster(config model.Config) *TTSBroadcaster {
	baseURL, err := url.Parse(config.TTSBroadcaster.Host)
	if err != nil {
		slog.Error("音声合成サーバのURLの解析に失敗しました", "error", err)
		baseURL = &url.URL{
			Scheme: "http",
			Host:   "localhost:50021",
		}
	}
	return &TTSBroadcaster{
		config:  config,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: config.TTSBroadcaster.Timeout,
		},
		streams: make(map[string]*Stream),
	}
}

func (t *TTSBroadcaster) Start() {
	if _, err := os.Stat(t.config.TTSBroadcaster.SegmentDir); os.IsNotExist(err) {
		os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755)
	}
	t.cleanupSegments()

	if err := t.buildSilenceTemplate(); err != nil {
		return
	}

	go t.streamManager()
}

func (t *TTSBroadcaster) getStream(id string) *Stream {
	t.streamsMu.RLock()
	stream, exists := t.streams[id]
	t.streamsMu.RUnlock()

	if exists {
		return stream
	}

	t.streamsMu.Lock()
	defer t.streamsMu.Unlock()

	if stream, exists = t.streams[id]; exists {
		return stream
	}

	stream = &Stream{
		isStreaming:     false,
		lastSegmentTime: time.Now(),
		segmentCounter:  0,
	}

	streamDir := filepath.Join(t.config.TTSBroadcaster.SegmentDir, id)
	if _, err := os.Stat(streamDir); os.IsNotExist(err) {
		os.MkdirAll(streamDir, 0755)
	}

	playlist, err := m3u8.NewMediaPlaylist(math.MaxInt16, math.MaxInt16)
	if err != nil {
		slog.Error("プレイリストの作成に失敗しました", "error", err, "id", id)
		return nil
	}

	playlist.TargetDuration = float64(t.config.TTSBroadcaster.TargetDuration.Seconds())
	playlist.SetVersion(3)
	playlist.Closed = false
	stream.playlist = playlist

	for range t.config.TTSBroadcaster.MinBufferSegments {
		t.addSilenceSegment(id, stream)
	}

	t.streams[id] = stream
	return stream
}

func (t *TTSBroadcaster) cleanupSegments() {
	if err := os.RemoveAll(t.config.TTSBroadcaster.SegmentDir); err != nil {
		slog.Error("セグメントディレクトリの削除に失敗しました", "error", err)
		return
	}
	slog.Info("セグメントディレクトリのクリーンアップが完了しました")
	if err := os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755); err != nil {
		slog.Error("セグメントディレクトリの作成に失敗しました", "error", err)
		return
	}
}

func (t *TTSBroadcaster) getSegmentDir(id string) string {
	return filepath.Join(t.config.TTSBroadcaster.SegmentDir, id)
}

func (t *TTSBroadcaster) buildSilenceTemplate() error {
	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)
	args := append(
		t.config.TTSBroadcaster.SilenceArgs,
		"-t",
		fmt.Sprintf("%f", t.config.TTSBroadcaster.TargetDuration.Seconds()),
		silenceTemplatePath,
	)
	cmd := exec.Command(t.config.TTSBroadcaster.FfmpegPath, args...)

	slog.Debug("無音セグメントを作成します",
		"command", t.config.TTSBroadcaster.FfmpegPath,
		"args", strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("無音セグメントの作成に失敗しました",
			"error", err,
			"output", string(output))
		return err
	}

	slog.Info("無音セグメントの作成が完了しました")
	return nil
}

func (t *TTSBroadcaster) addSilenceSegment(id string, stream *Stream) {
	stream.segmentsMu.Lock()
	silenceSegmentName := fmt.Sprintf("segment_%d.ts", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	streamDir := t.getSegmentDir(id)
	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)
	silenceSegmentPath := filepath.Join(streamDir, silenceSegmentName)

	if err := t.copyFile(silenceTemplatePath, silenceSegmentPath); err != nil {
		slog.Error("無音セグメントのコピーに失敗しました", "error", err, "id", id)
		return
	}

	stream.playlistMu.Lock()
	defer stream.playlistMu.Unlock()

	if err := stream.playlist.AppendSegment(&m3u8.MediaSegment{
		URI:      silenceSegmentName,
		Duration: t.config.TTSBroadcaster.TargetDuration.Seconds(),
	}); err != nil {
		slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
	}

	t.writePlaylist(id, stream)
}

func (t *TTSBroadcaster) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (t *TTSBroadcaster) writePlaylist(id string, stream *Stream) {
	streamDir := t.getSegmentDir(id)
	playlistPath := filepath.Join(streamDir, "playlist.m3u8")

	if err := os.MkdirAll(streamDir, 0755); err != nil {
		slog.Error("プレイリストディレクトリの作成に失敗しました", "error", err, "id", id)
		return
	}

	if err := os.WriteFile(playlistPath, stream.playlist.Encode().Bytes(), 0644); err != nil {
		slog.Error("プレイリストの書き込みに失敗しました", "error", err, "id", id)
	}
}

func (t *TTSBroadcaster) convertWavToSegment(data []byte, id string, baseName string) ([]string, error) {
	streamDir := t.getSegmentDir(id)
	tempWavFile, err := os.CreateTemp(streamDir, "temp-*.wav")
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

	duration, err := t.getDuration(tempWavPath)
	if err != nil {
		return nil, err
	}

	if duration <= t.config.TTSBroadcaster.TargetDuration.Seconds() {
		outputPath := filepath.Join(streamDir, baseName)
		args := []string{"-i", tempWavPath}
		args = append(args, t.config.TTSBroadcaster.ConvertArgs...)
		args = append(args, outputPath)

		slog.Debug("生成した音声をセグメントに変換します",
			"command", t.config.TTSBroadcaster.FfmpegPath,
			"args", strings.Join(args, " "),
			"id", id,
			"input", tempWavPath,
			"output", outputPath)

		cmd := exec.Command(t.config.TTSBroadcaster.FfmpegPath, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Error("生成した音声の変換に失敗しました",
				"error", err,
				"output", string(output),
				"id", id)
			return nil, err
		}

		slog.Info("生成した音声の変換が完了しました", "id", id)
		return []string{baseName}, nil
	}

	return t.splitWavIntoSegments(tempWavPath, id, baseName, duration)
}

func (t *TTSBroadcaster) getDuration(path string) (float64, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, err
	}
	args := append(t.config.TTSBroadcaster.DurationArgs, path)

	slog.Debug("音声の長さを取得します",
		"command", t.config.TTSBroadcaster.FfprobePath,
		"args", strings.Join(args, " "),
		"path", path)

	cmd := exec.Command(
		t.config.TTSBroadcaster.FfprobePath,
		args...,
	)
	output, err := cmd.Output()
	if err != nil {
		slog.Error("音声の長さの取得に失敗しました",
			"error", err,
			"path", path)
		return 0, err
	}

	var duration float64
	fmt.Sscanf(string(output), "%f", &duration)

	slog.Info("音声の長さを取得しました",
		"path", path,
		"duration", duration,
		"output", string(output))
	return duration, nil
}

func (t *TTSBroadcaster) splitWavIntoSegments(wavPath string, id string, baseName string, totalDuration float64) ([]string, error) {
	streamDir := t.getSegmentDir(id)
	baseNameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	segmentNames := []string{}
	tempDir, err := os.MkdirTemp(t.config.TTSBroadcaster.TempDir, "split-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	aacPath := filepath.Join(tempDir, "input.aac")
	convertArgs := []string{"-i", wavPath}
	convertArgs = append(convertArgs, t.config.TTSBroadcaster.PreConvertArgs...)
	convertArgs = append(convertArgs, aacPath)

	slog.Debug("生成した音声の事前変換を開始します",
		"command", t.config.TTSBroadcaster.FfmpegPath,
		"args", strings.Join(convertArgs, " "),
		"id", id,
		"input", wavPath,
		"output", aacPath)

	convertCmd := exec.Command(t.config.TTSBroadcaster.FfmpegPath, convertArgs...)
	output, err := convertCmd.CombinedOutput()
	if err != nil {
		slog.Error("生成した音声の事前変換に失敗しました",
			"error", err,
			"output", string(output),
			"id", id)
		return nil, err
	}

	slog.Info("生成した音声の事前変換が完了しました", "id", id)

	segmentCount := int(math.Ceil(totalDuration / t.config.TTSBroadcaster.TargetDuration.Seconds()))
	segmentDuration := t.config.TTSBroadcaster.TargetDuration.Seconds()

	for i := range segmentCount {
		segmentName := fmt.Sprintf("%s_part%03d.ts", baseNameWithoutExt, i)
		segmentPath := filepath.Join(streamDir, segmentName)
		startTime := float64(i) * segmentDuration
		duration := segmentDuration
		if i == segmentCount-1 {
			duration = totalDuration - startTime
		}
		segmentArgs := []string{"-i", aacPath, "-ss", fmt.Sprintf("%f", startTime), "-t", fmt.Sprintf("%f", duration)}
		segmentArgs = append(segmentArgs, t.config.TTSBroadcaster.SplitArgs...)
		segmentArgs = append(segmentArgs, segmentPath)

		slog.Debug("生成した音声のセグメント分割を開始します",
			"command", t.config.TTSBroadcaster.FfmpegPath,
			"args", strings.Join(segmentArgs, " "),
			"id", id,
			"segment", i,
			"startTime", startTime,
			"duration", duration,
			"output", segmentPath)

		segmentCmd := exec.Command(t.config.TTSBroadcaster.FfmpegPath, segmentArgs...)
		output, err := segmentCmd.CombinedOutput()
		if err != nil {
			slog.Error("生成した音声のセグメント分割に失敗しました",
				"error", err,
				"output", string(output),
				"id", id,
				"segment", i)
			return nil, err
		}

		slog.Info("生成した音声のセグメント分割が完了しました", "id", id, "segment", i)

		segmentNames = append(segmentNames, segmentName)
	}

	slog.Info("すべてのセグメントの分割が完了しました", "id", id, "size", len(segmentNames))
	return segmentNames, nil
}

func (t *TTSBroadcaster) streamManager() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		t.streamsMu.RLock()
		streamIDs := make([]string, 0, len(t.streams))
		for id := range t.streams {
			streamIDs = append(streamIDs, id)
		}
		t.streamsMu.RUnlock()

		for _, id := range streamIDs {
			t.streamsMu.RLock()
			stream, exists := t.streams[id]
			t.streamsMu.RUnlock()

			if !exists {
				continue
			}

			stream.streamingMu.Lock()
			isStreaming := stream.isStreaming
			lastTime := stream.lastSegmentTime
			stream.streamingMu.Unlock()

			if !isStreaming {
				elapsed := time.Since(lastTime).Seconds()

				if elapsed >= t.config.TTSBroadcaster.TargetDuration.Seconds()*0.8 {
					t.addSilenceSegment(id, stream)
					stream.streamingMu.Lock()
					stream.lastSegmentTime = time.Now()
					stream.streamingMu.Unlock()
				}
			}
		}
		// t.cleanupOldStreams()
	}
}

func (t *TTSBroadcaster) HandlePlaylist(c *gin.Context) {
	id := c.Param("id")
	if id == "" || strings.ContainsAny(id, "/\\") {
		c.Status(http.StatusBadRequest)
		return
	}

	if !isValidID(id) {
		c.Status(http.StatusBadRequest)
		return
	}

	streamDir := t.getSegmentDir(id)
	playlistPath := filepath.Join(streamDir, "playlist.m3u8")

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
	id := c.Param("id")
	if id == "" || strings.ContainsAny(id, "/\\") {
		c.Status(http.StatusBadRequest)
		return
	}

	if !isValidID(id) {
		c.Status(http.StatusBadRequest)
		return
	}

	segment := c.Param("segment")
	segmentName := strings.TrimPrefix(segment, "/")

	if !strings.HasSuffix(segmentName, ".ts") || strings.ContainsAny(segmentName, "/\\") {
		c.Status(http.StatusNotFound)
		return
	}

	if !isValidSegmentName(segmentName) {
		c.Status(http.StatusBadRequest)
		return
	}

	streamDir := t.getSegmentDir(id)
	segmentPath := filepath.Join(streamDir, segmentName)

	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
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

func isValidID(id string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", id)
	return match
}

func isValidSegmentName(name string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+\\.ts$", name)
	return match
}

func (t *TTSBroadcaster) BroadcastText(id string, text string, speaker int) {
	stream := t.getStream(id)
	if stream == nil {
		return
	}

	stream.streamingMu.Lock()
	stream.isStreaming = true
	stream.streamingMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), t.config.TTSBroadcaster.Timeout)
		defer cancel()

		audioQueryCh := t.fetchAudioQueryAsync(ctx, text, speaker)
		if err := t.processTextToSpeech(ctx, audioQueryCh, id, stream, speaker); err != nil {
			slog.Error("音声合成に失敗しました", "error", err, "id", id)
		}

		stream.streamingMu.Lock()
		stream.isStreaming = false
		stream.lastSegmentTime = time.Now()
		stream.streamingMu.Unlock()
	}()
}

func (t *TTSBroadcaster) fetchAudioQueryAsync(ctx context.Context, text string, speaker int) <-chan []byte {
	resultCh := make(chan []byte, 1)

	go func() {
		defer close(resultCh)

		baseURL := *t.baseURL
		baseURL.Path = "/audio_query"
		params := url.Values{}
		params.Add("speaker", fmt.Sprintf("%d", speaker))
		params.Add("text", text)
		baseURL.RawQuery = params.Encode()
		queryURL := baseURL.String()

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

func (t *TTSBroadcaster) processTextToSpeech(ctx context.Context, audioQueryCh <-chan []byte, id string, stream *Stream, speaker int) error {
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

	baseURL := *t.baseURL
	baseURL.Path = "/synthesis"
	params := url.Values{}
	params.Add("speaker", fmt.Sprintf("%d", speaker))
	baseURL.RawQuery = params.Encode()
	queryURL := baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, bytes.NewBuffer(queryParams))
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

	stream.segmentsMu.Lock()
	baseSegmentName := fmt.Sprintf("segment_%d.ts", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	segmentNames, err := t.convertWavToSegment(wavData, id, baseSegmentName)
	if err != nil {
		return err
	}

	t.addSegmentsToPlaylist(id, stream, segmentNames)
	return nil
}

func (t *TTSBroadcaster) addSegmentsToPlaylist(id string, stream *Stream, segmentNames []string) {
	if len(segmentNames) == 0 {
		return
	}

	streamDir := t.getSegmentDir(id)
	stream.playlistMu.Lock()
	defer stream.playlistMu.Unlock()

	for _, segmentName := range segmentNames {
		duration, err := t.getDuration(filepath.Join(streamDir, segmentName))
		if err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
			continue
		}

		if err := stream.playlist.AppendSegment(&m3u8.MediaSegment{
			URI:      segmentName,
			Duration: duration,
		}); err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
		}
	}

	t.writePlaylist(id, stream)
}

func (t *TTSBroadcaster) CleanupStream(id string) {
	t.streamsMu.Lock()
	delete(t.streams, id)
	t.streamsMu.Unlock()

	streamDir := t.getSegmentDir(id)
	os.RemoveAll(streamDir)
}
