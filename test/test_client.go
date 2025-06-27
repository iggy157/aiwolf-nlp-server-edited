package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/gorilla/websocket"
)

type TestClient struct {
	conn           *websocket.Conn
	done           chan struct{}
	name           string
	request        model.Request
	info           map[string]any
	setting        map[string]any
	talkHistory    []any
	whisperHistory []any
	role           model.Role
	handlers       map[model.Request]func(tc TestClient) (string, error)
}

func NewTestClient(t *testing.T, u url.URL, name string, handlers map[model.Request]func(tc TestClient) (string, error)) (*TestClient, error) {
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %v", err)
	}
	client := &TestClient{
		conn:     c,
		done:     make(chan struct{}),
		name:     name,
		handlers: handlers,
	}
	go client.listen(t)
	return client, nil
}

func HandleTarget(tc TestClient) (string, error) {
	if statusMap, exists := tc.info["status_map"].(map[string]any); exists {
		for k, v := range statusMap {
			if k == tc.info["agent"].(string) {
				continue
			}
			if v == model.S_ALIVE.String() {
				return k, nil
			}
		}
		return "", errors.New("target not found")
	}
	return "", errors.New("status_map not found")
}

func (tc *TestClient) listen(t *testing.T) {
	defer close(tc.done)
	for {
		_, message, err := tc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				t.Logf("connection closed: %v", err)
				return
			}
			t.Logf("read: %v", err)
			return
		}
		t.Logf("recv: %s", message)

		var recv map[string]any
		if err := json.Unmarshal(message, &recv); err != nil {
			t.Logf("unmarshal: %v", err)
			continue
		}

		req := model.RequestFromString(recv["request"].(string))
		resp, err := tc.handleRequest(req, recv)
		if err != nil {
			t.Error(err)
		}
		tc.request = req

		if req.RequireResponse {
			err = tc.conn.WriteMessage(websocket.TextMessage, []byte(resp))
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Logf("connection closed: %v", err)
					return
				}
				t.Logf("write: %v", err)
				continue
			}
			t.Logf("send: %s", resp)
		}
	}
}

func (tc *TestClient) setInfo(recv map[string]any) error {
	if info, exists := recv["info"].(map[string]any); exists {
		tc.info = info
		if tc.role.String() == "" {
			if roleMap, exists := info["role_map"].(map[string]any); exists {
				for _, v := range roleMap {
					tc.role = model.RoleFromString(v.(string))
					break
				}
			}
		}
	} else {
		return errors.New("info not found")
	}
	return nil
}

func (tc *TestClient) setSetting(recv map[string]any) error {
	if setting, exists := recv["setting"].(map[string]any); exists {
		tc.setting = setting
	} else {
		return errors.New("setting not found")
	}
	return nil
}

func (tc *TestClient) handleRequest(request model.Request, recv map[string]any) (string, error) {
	switch request {
	case model.R_NAME:
	case model.R_INITIALIZE, model.R_DAILY_INITIALIZE:
		err := tc.setInfo(recv)
		if err != nil {
			return "", err
		}
		err = tc.setSetting(recv)
		if err != nil {
			return "", err
		}
	case model.R_VOTE, model.R_DIVINE, model.R_GUARD:
		err := tc.setInfo(recv)
		if err != nil {
			return "", err
		}
	case model.R_DAILY_FINISH, model.R_TALK, model.R_WHISPER, model.R_ATTACK:
		err := tc.setInfo(recv)
		if err != nil {
			return "", err
		}
		if request == model.R_TALK || request == model.R_DAILY_FINISH {
			if talkHistory, exists := recv["talk_history"].([]any); exists {
				tc.talkHistory = talkHistory
			} else {
				return "", errors.New("talk_history not found")
			}
		}
		if request == model.R_WHISPER || request == model.R_ATTACK || (request == model.R_DAILY_FINISH && tc.role == model.R_WEREWOLF) {
			if whisperHistory, exists := recv["whisper_history"].([]any); exists {
				tc.whisperHistory = whisperHistory
			} else {
				return "", errors.New("whisper_history not found")
			}
		}
	case model.R_FINISH:
		err := tc.setInfo(recv)
		if err != nil {
			return "", err
		}
	}
	if handler, exists := tc.handlers[request]; exists {
		resp, err := handler(*tc)
		if err != nil {
			return "", fmt.Errorf("handle %s: %v", request.String(), err)
		}
		return resp, nil
	} else {
		return "", nil
	}
}

func (tc *TestClient) Close() {
	tc.conn.Close()
	select {
	case <-tc.done:
	case <-time.After(time.Second):
	}
}
