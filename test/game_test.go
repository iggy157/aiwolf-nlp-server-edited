package test

import (
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/core"
	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

const WebSocketExternalHost = "0.0.0.0"
const TestClientName = "aiwolf-nlp-viewer"

func launchAsyncServer(config *model.Config) url.URL {
	if _, exists := os.LookupEnv("GITHUB_ACTIONS"); exists {
		config.Server.WebSocket.Host = WebSocketExternalHost
	}
	port := getAvailableTcpPort()
	config.Server.WebSocket.Port = port
	go func() {
		server, err := core.NewServer(*config)
		if err != nil {
			return
		}
		server.Run()
	}()
	time.Sleep(5 * time.Second)
	return url.URL{Scheme: "ws", Host: config.Server.WebSocket.Host + ":" + strconv.Itoa(config.Server.WebSocket.Port), Path: "/ws"}
}

func getAvailableTcpPort() int {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	port := rand.Intn(65535-49152+1) + 49152
	for {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			listener.Close()
			break
		}
		port = rand.Intn(65535-49152+1) + 49152
	}
	return port
}

func TestGame(t *testing.T) {
	config, err := model.LoadFromPath("../config/debug.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	u := launchAsyncServer(config)
	t.Logf("Connecting to %s", u.String())

	clients := make([]*TestClient, config.Game.AgentCount)
	for i := range config.Game.AgentCount {
		client, err := NewRandomTestClient(t, u, TestClientName)
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
