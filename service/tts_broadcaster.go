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
	"path/filepath"
	"sync"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
	"github.com/grafov/m3u8"
)

const (
	silenceTemplateFile = "silence.ts"
	playlistFile        = "playlist.m3u8"
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
	if err := os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755); err != nil {
		slog.Error("セグメントディレクトリの作成に失敗しました", "error", err)
		return
	}
	t.cleanupSegments()
	outputPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, silenceTemplateFile)
	if err := util.BuildSilenceTemplate(
		t.config.TTSBroadcaster.FfmpegPath,
		t.config.TTSBroadcaster.SilenceArgs,
		t.config.TTSBroadcaster.TargetDuration.Seconds(),
		outputPath,
	); err != nil {
		slog.Error("無音テンプレートの構築に失敗しました", "error", err)
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
	return nil
}

func (t *TTSBroadcaster) CreateStream(id string) {
	t.streamsMu.Lock()
	defer t.streamsMu.Unlock()
	if _, exists := t.streams[id]; exists {
		return
	}

	stream := &Stream{
		isStreaming:     false,
		lastSegmentTime: time.Now(),
		segmentCounter:  0,
	}

	streamDir := t.getSegmentDir(id)
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		slog.Error("ストリームディレクトリの作成に失敗しました", "error", err, "id", id)
		return
	}

	playlist, err := m3u8.NewMediaPlaylist(math.MaxInt16, math.MaxInt16)
	if err != nil {
		slog.Error("プレイリストの作成に失敗しました", "error", err, "id", id)
		return
	}
	playlist.TargetDuration = float64(t.config.TTSBroadcaster.TargetDuration.Seconds())
	playlist.SetVersion(3)
	playlist.Closed = false
	stream.playlist = playlist

	t.writePlaylist(id, stream)
	t.streams[id] = stream
	slog.Info("ストリームを作成しました", "id", id)
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
	cleanID := filepath.Base(filepath.Clean(id))
	return filepath.Join(t.config.TTSBroadcaster.SegmentDir, cleanID)
}

func (t *TTSBroadcaster) addSilenceSegment(id string, stream *Stream) {
	stream.segmentsMu.Lock()
	silenceSegmentName := fmt.Sprintf("segment_%d.ts", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	streamDir := t.getSegmentDir(id)
	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, silenceTemplateFile)
	silenceSegmentPath := filepath.Join(streamDir, silenceSegmentName)

	if err := util.CopyFile(silenceTemplatePath, silenceSegmentPath); err != nil {
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

func (t *TTSBroadcaster) writePlaylist(id string, stream *Stream) {
	streamDir := t.getSegmentDir(id)
	playlistPath := filepath.Join(streamDir, playlistFile)
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		slog.Error("プレイリストディレクトリの作成に失敗しました", "error", err, "id", id)
		return
	}
	if err := os.WriteFile(playlistPath, stream.playlist.Encode().Bytes(), 0644); err != nil {
		slog.Error("プレイリストの書き込みに失敗しました", "error", err, "id", id)
	}
}

func (t *TTSBroadcaster) streamManager() {
	ticker := time.NewTicker(200 * time.Millisecond)
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
				elapsed := time.Since(lastTime)
				nextSegmentThreshold := time.Duration(float64(t.config.TTSBroadcaster.TargetDuration) * 0.8)

				if elapsed >= nextSegmentThreshold {
					t.addSilenceSegment(id, stream)
					stream.streamingMu.Lock()
					stream.lastSegmentTime = time.Now()
					stream.streamingMu.Unlock()
				}
			}
		}
	}
}

func (t *TTSBroadcaster) BroadcastText(id string, text string, speaker int) {
	if text == "SKIP" {
		text = "スキップ"
	} else if text == "OVER" {
		text = "オーバー"
	}
	if t.config.TTSBroadcaster.Async {
		t.broadcastTextAsync(id, text, speaker)
	} else {
		t.broadcastText(id, text, speaker)
	}
}

func (t *TTSBroadcaster) broadcastTextAsync(id string, text string, speaker int) {
	stream := t.getStream(id)
	if stream == nil {
		return
	}

	stream.streamingMu.Lock()
	stream.isStreaming = true
	stream.streamingMu.Unlock()

	go func() {
		defer func() {
			stream.streamingMu.Lock()
			stream.isStreaming = false
			stream.lastSegmentTime = time.Now()
			stream.streamingMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), t.config.TTSBroadcaster.Timeout)
		defer cancel()

		audioQuery, err := t.fetchAudioQuery(ctx, text, speaker)
		if err != nil {
			slog.Error("オーディオクエリの取得に失敗しました", "error", err, "id", id)
			return
		}

		if _, err := t.synthesizeAndProcessAudio(ctx, audioQuery, id, stream, speaker); err != nil {
			slog.Error("音声合成に失敗しました", "error", err, "id", id)
		}
	}()
}

func (t *TTSBroadcaster) broadcastText(id string, text string, speaker int) {
	stream := t.getStream(id)
	if stream == nil {
		return
	}

	stream.streamingMu.Lock()
	stream.isStreaming = true
	stream.streamingMu.Unlock()

	defer func() {
		stream.streamingMu.Lock()
		stream.isStreaming = false
		stream.lastSegmentTime = time.Now()
		stream.streamingMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), t.config.TTSBroadcaster.Timeout)
	defer cancel()

	audioQuery, err := t.fetchAudioQuery(ctx, text, speaker)
	if err != nil {
		slog.Error("オーディオクエリの取得に失敗しました", "error", err, "id", id)
		return
	}

	duration, err := t.synthesizeAndProcessAudio(ctx, audioQuery, id, stream, speaker)
	if err != nil {
		slog.Error("音声合成に失敗しました", "error", err, "id", id)
		return
	}
	time.Sleep(time.Duration(duration * float64(time.Second)))
}

func (t *TTSBroadcaster) fetchAudioQuery(ctx context.Context, text string, speaker int) ([]byte, error) {
	baseURL := *t.baseURL
	baseURL.Path = "/audio_query"

	params := url.Values{}
	params.Add("speaker", fmt.Sprintf("%d", speaker))
	params.Add("text", text)
	baseURL.RawQuery = params.Encode()
	queryURL := baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("オーディオクエリリクエスト作成に失敗しました: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("オーディオクエリ送信に失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("オーディオクエリエラー: ステータスコード %d", resp.StatusCode)
	}

	queryParams, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("オーディオクエリ読み取りに失敗しました: %w", err)
	}

	return queryParams, nil
}

func (t *TTSBroadcaster) synthesizeAndProcessAudio(ctx context.Context, queryParams []byte, id string, stream *Stream, speaker int) (float64, error) {
	baseURL := *t.baseURL
	baseURL.Path = "/synthesis"
	params := url.Values{}
	params.Add("speaker", fmt.Sprintf("%d", speaker))
	baseURL.RawQuery = params.Encode()
	queryURL := baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, bytes.NewBuffer(queryParams))
	if err != nil {
		return 0, fmt.Errorf("合成リクエスト作成に失敗しました: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("合成リクエスト送信に失敗しました: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("合成エラー: ステータスコード %d", resp.StatusCode)
	}

	wavData, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("合成データ読み取りに失敗しました: %w", err)
	}

	stream.segmentsMu.Lock()
	baseName := fmt.Sprintf("segment_%d", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	segmentParams := util.ConvertWavToSegmentParams{
		FfmpegPath:      t.config.TTSBroadcaster.FfmpegPath,
		FfprobePath:     t.config.TTSBroadcaster.FfprobePath,
		DurationArgs:    t.config.TTSBroadcaster.DurationArgs,
		ConvertArgs:     t.config.TTSBroadcaster.ConvertArgs,
		PreConvertArgs:  t.config.TTSBroadcaster.PreConvertArgs,
		SplitArgs:       t.config.TTSBroadcaster.SplitArgs,
		TempDir:         t.config.TTSBroadcaster.TempDir,
		SegmentDuration: t.config.TTSBroadcaster.TargetDuration.Seconds(),
		Data:            wavData,
		BaseDir:         t.getSegmentDir(id),
		BaseName:        baseName,
	}

	segmentNames, err := util.ConvertWavToSegment(segmentParams)
	if err != nil {
		return 0, fmt.Errorf("WAVからセグメントへの変換に失敗しました: %w", err)
	}

	return t.addSegmentsToPlaylist(id, stream, segmentNames), nil
}

func (t *TTSBroadcaster) addSegmentsToPlaylist(id string, stream *Stream, segmentNames []string) float64 {
	if len(segmentNames) == 0 {
		return 0
	}

	streamDir := t.getSegmentDir(id)
	stream.playlistMu.Lock()
	defer stream.playlistMu.Unlock()

	var totalDuration float64

	for _, segmentName := range segmentNames {
		segmentPath := filepath.Join(streamDir, segmentName)

		duration, err := util.GetDuration(t.config.TTSBroadcaster.FfprobePath, t.config.TTSBroadcaster.DurationArgs, segmentPath)
		if err != nil {
			slog.Error("セグメント再生時間の取得に失敗しました", "error", err, "id", id, "segment", segmentName)
			duration = t.config.TTSBroadcaster.TargetDuration.Seconds()
		}
		totalDuration += duration

		if err := stream.playlist.AppendSegment(&m3u8.MediaSegment{
			URI:      segmentName,
			Duration: duration,
		}); err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id, "segment", segmentName)
		}
	}

	t.writePlaylist(id, stream)
	return totalDuration
}
