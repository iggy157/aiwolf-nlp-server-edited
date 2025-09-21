package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
)

type RealtimeBroadcaster struct {
	config model.RealtimeBroadcasterConfig
	data   sync.Map
}

type RealtimeBroadcasterLog struct {
	id        string
	filename  string
	agents    []any
	logs      []string
	logsMu    sync.Mutex
	updatedAt time.Time
}

func NewRealtimeBroadcaster(config model.Config) *RealtimeBroadcaster {
	rb := &RealtimeBroadcaster{
		config: config.RealtimeBroadcaster,
	}
	if err := os.MkdirAll(rb.config.OutputDir, 0755); err != nil {
		slog.Error("出力ディレクトリの作成に失敗しました", "error", err)
		return nil
	}
	if err := os.WriteFile(filepath.Join(rb.config.OutputDir, "games.json"), []byte("[]"), 0644); err != nil {
		slog.Error("ゲーム一覧ファイルの初期化に失敗しました", "error", err)
		return nil
	}
	slog.Info("リアルタイムブロードキャスターを初期化しました", "output_dir", rb.config.OutputDir)
	return rb
}

func (rb *RealtimeBroadcaster) TrackStartGame(id string, agents []*model.Agent) {
	agentData := make([]any, 0, len(agents))
	teamNames := make([]string, 0, len(agents))

	for _, agent := range agents {
		agentInfo := map[string]any{
			"idx":  agent.Idx,
			"team": agent.TeamName,
			"name": agent.OriginalName,
			"role": agent.Role,
		}
		agentData = append(agentData, agentInfo)
		teamNames = append(teamNames, agent.TeamName)
	}

	sort.Strings(teamNames)
	filename := strings.ReplaceAll(rb.config.Filename, "{game_id}", id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))
	filename = strings.ReplaceAll(filename, "{teams}", strings.Join(teamNames, "_"))

	gameLog := &RealtimeBroadcasterLog{
		id:        id,
		filename:  filename,
		agents:    agentData,
		logs:      make([]string, 0),
		updatedAt: time.Now(),
	}

	rb.data.Store(id, gameLog)
}

func (rb *RealtimeBroadcaster) TrackEndGame(id string) {
	if gameLogInterface, exists := rb.data.Load(id); exists {
		gameLog := gameLogInterface.(*RealtimeBroadcasterLog)
		gameLog.logsMu.Lock()
		logs := make([]string, len(gameLog.logs))
		copy(logs, gameLog.logs)
		filename := gameLog.filename
		gameLog.logsMu.Unlock()

		rb.writeGameFile(filename, logs)
		rb.writeGamesListFile()
		rb.data.Delete(id)
	}
}

func (rb *RealtimeBroadcaster) Broadcast(packet model.BroadcastPacket) {
	data, err := json.Marshal(packet)
	if err != nil {
		slog.Error("パケットのJSON化に失敗しました", "error", err)
		return
	}

	if gameLogInterface, exists := rb.data.Load(packet.Id); exists {
		gameLog := gameLogInterface.(*RealtimeBroadcasterLog)
		gameLog.logsMu.Lock()
		gameLog.logs = append(gameLog.logs, string(data))
		gameLog.updatedAt = time.Now()
		logs := make([]string, len(gameLog.logs))
		copy(logs, gameLog.logs)
		filename := gameLog.filename
		gameLog.logsMu.Unlock()

		rb.writeGameFile(filename, logs)
		rb.writeGamesListFile()
		slog.Info("JSONLファイルにブロードキャストを保存しました", "game_id", packet.Id)
	}
}

func (rb *RealtimeBroadcaster) writeGamesListFile() {
	type Item struct {
		ID        string    `json:"id"`
		Filename  string    `json:"filename"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	items := make([]Item, 0)
	rb.data.Range(func(_, value any) bool {
		gameLog := value.(*RealtimeBroadcasterLog)
		item := Item{
			ID:        gameLog.id,
			Filename:  gameLog.filename,
			UpdatedAt: gameLog.updatedAt,
		}
		items = append(items, item)
		return true
	})

	data, err := json.Marshal(items)
	if err != nil {
		slog.Error("ゲーム一覧のJSON生成に失敗しました", "error", err)
		return
	}
	filePath := filepath.Join(rb.config.OutputDir, "games.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		slog.Error("ゲーム一覧ファイルの作成に失敗しました", "error", err)
		return
	}
	slog.Info("ゲーム一覧ファイルを更新しました", "path", filePath)
}

func (rb *RealtimeBroadcaster) writeGameFile(filename string, logs []string) {
	filePath := filepath.Join(rb.config.OutputDir, fmt.Sprintf("%s.jsonl", filename))
	content := strings.Join(logs, "\n")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		slog.Error("ゲームファイルの保存に失敗しました", "error", err, "path", filePath)
		return
	}
	slog.Info("ゲームファイルを保存しました", "path", filePath)
}
