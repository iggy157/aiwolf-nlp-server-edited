package core

import (
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
)

type WaitingRoom struct {
	agentCount  int
	selfMatch   bool
	connections sync.Map
}

func NewWaitingRoom(config model.Config) *WaitingRoom {
	return &WaitingRoom{
		agentCount: config.Game.AgentCount,
		selfMatch:  config.Matching.SelfMatch,
	}
}

func (wr *WaitingRoom) AddConnection(team string, connection model.Connection) {
	value, _ := wr.connections.LoadOrStore(team, []model.Connection{})
	connections := value.([]model.Connection)

	updatedConnections := append(connections, connection)
	wr.connections.Store(team, updatedConnections)

	slog.Info("新しいクライアントが待機部屋に追加されました", "team", team, "remote_addr", connection.Conn.RemoteAddr().String())
}

func (wr *WaitingRoom) GetConnectionsWithMatchOptimizer(matches []map[model.Role][]string) (map[model.Role][]model.Connection, error) {
	var roleMapConns = make(map[model.Role][]model.Connection)

	if len(matches) == 0 {
		return nil, errors.New("スケジュールされたマッチがありません")
	}

	readyMatch := map[model.Role][]string{}
	for _, match := range matches {
		isMatchReady := true
		for _, teams := range match {
			for _, team := range teams {
				value, exists := wr.connections.Load(team)
				if !exists {
					isMatchReady = false
					break
				}
				connections := value.([]model.Connection)
				if len(connections) == 0 {
					isMatchReady = false
					break
				}
			}
			if !isMatchReady {
				break
			}
		}

		if isMatchReady {
			readyMatch = match
			break
		}
	}

	if len(readyMatch) == 0 {
		return nil, errors.New("スケジュールされたマッチ内に不足しているチームがあります")
	}
	slog.Info("スケジュールされたマッチの接続を取得しました")

	for role, teams := range readyMatch {
		for _, team := range teams {
			value, exists := wr.connections.Load(team)
			if !exists {
				continue
			}
			connections := value.([]model.Connection)

			roleMapConns[role] = append(roleMapConns[role], connections[0])

			if len(connections) > 1 {
				wr.connections.Store(team, connections[1:])
			} else {
				wr.connections.Delete(team)
			}
		}
	}
	return roleMapConns, nil
}

func (wr *WaitingRoom) GetConnections() ([]model.Connection, error) {
	connections := []model.Connection{}
	ready := false

	if wr.selfMatch {
		wr.connections.Range(func(key, value any) bool {
			team := key.(string)
			conns := value.([]model.Connection)

			if len(conns) >= wr.agentCount {
				connections = append(connections, conns[:wr.agentCount]...)

				if len(conns) > wr.agentCount {
					wr.connections.Store(team, conns[wr.agentCount:])
				} else {
					wr.connections.Delete(team)
				}
				ready = true
				return false
			}
			return true
		})
	} else {
		var teams []string
		wr.connections.Range(func(key, value any) bool {
			team := key.(string)
			conns := value.([]model.Connection)
			if len(conns) > 0 {
				teams = append(teams, team)
			}
			return true
		})

		if len(teams) >= wr.agentCount {
			rand.Shuffle(len(teams), func(i, j int) {
				teams[i], teams[j] = teams[j], teams[i]
			})

			for _, team := range teams[:wr.agentCount] {
				value, exists := wr.connections.Load(team)
				if !exists {
					continue
				}
				conns := value.([]model.Connection)
				if len(conns) == 0 {
					continue
				}

				connections = append(connections, conns[0])

				if len(conns) > 1 {
					wr.connections.Store(team, conns[1:])
				} else {
					wr.connections.Delete(team)
				}
			}
			ready = true
		}
	}

	if !ready {
		return nil, errors.New("待機部屋内の接続が不足しています")
	}
	slog.Info("マッチの接続を取得しました")
	return connections, nil
}
