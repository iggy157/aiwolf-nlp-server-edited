package test

import (
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/core"
	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/joho/godotenv"
)

const WebSocketExternalHost = "0.0.0.0"

func TestGame(t *testing.T) {
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); !exists {
		godotenv.Load("../config/.env")
	}

	config, err := model.LoadFromPath("../config/debug.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); exists {
		config.Server.WebSocket.Host = WebSocketExternalHost
	}
	go func() {
		server, err := core.NewServer(*config)
		if err != nil {
			return
		}
		server.Run()
	}()
	time.Sleep(5 * time.Second)

	u := url.URL{Scheme: "ws", Host: config.Server.WebSocket.Host + ":" + strconv.Itoa(config.Server.WebSocket.Port), Path: "/ws"}
	t.Logf("Connecting to %s", u.String())

	names := make([]string, config.Game.AgentCount)
	for i := range config.Game.AgentCount {
		names[i] = "aiwolf-nlp-viewer"
	}

	clients := make([]*TestClient, config.Game.AgentCount)
	for i := range config.Game.AgentCount {
		client, err := NewRandomTestClient(t, u, names[i])
		if err != nil {
			t.Fatalf("Failed to create WebSocket client: %v", err)
		}
		clients[i] = client
		defer clients[i].Close()
	}

	for _, client := range clients {
		select {
		case <-client.done:
			t.Log("Connection closed")
		case <-time.After(5 * time.Minute):
			t.Fatalf("Timeout")
		}
	}

	time.Sleep(5 * time.Second)
	t.Log("Test completed successfully")
}

func TestManualGame(t *testing.T) {
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); !exists {
		godotenv.Load("../config/.env")
	}

	config, err := model.LoadFromPath("../config/debug.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); exists {
		config.Server.WebSocket.Host = WebSocketExternalHost
		return
	}
	go func() {
		server, err := core.NewServer(*config)
		if err != nil {
			return
		}
		server.Run()
	}()
	time.Sleep(5 * time.Second)

	u := url.URL{Scheme: "ws", Host: config.Server.WebSocket.Host + ":" + strconv.Itoa(config.Server.WebSocket.Port), Path: "/ws"}
	t.Logf("Connecting to %s", u.String())

	names := make([]string, config.Game.AgentCount-1)
	for i := range config.Game.AgentCount - 1 {
		names[i] = "aiwolf-nlp-viewer"
	}

	clients := make([]*TestClient, config.Game.AgentCount-1)
	for i := range config.Game.AgentCount - 1 {
		client, err := NewRandomTestClient(t, u, names[i])
		if err != nil {
			t.Fatalf("Failed to create WebSocket client: %v", err)
		}
		clients[i] = client
		defer clients[i].Close()
	}

	for _, client := range clients {
		<-client.done
		t.Log("Connection closed")
	}

	time.Sleep(5 * time.Second)
	t.Log("Test completed successfully")
}
