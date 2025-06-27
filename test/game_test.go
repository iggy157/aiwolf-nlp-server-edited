package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestFullGame(t *testing.T) {
	config, err := model.LoadFromPath("./config/full.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_VOTE:   HandleTarget,
		model.R_DIVINE: HandleTarget,
		model.R_GUARD:  HandleTarget,
		model.R_TALK: func(tc TestClient) (string, error) {
			return "Hello World!", nil
		},
		model.R_WHISPER: func(tc TestClient) (string, error) {
			return "Hello World!", nil
		},
		model.R_ATTACK: HandleTarget,
	}
	ExecuteGame(t, config, handlers)
}

func TestExecutionPhase(t *testing.T) {
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	targetName := "Player1"

	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_VOTE: func(tc TestClient) (string, error) {
			if tc.name != targetName {
				return targetName, nil
			}
			return "", nil
		},
		model.R_FINISH: func(tc TestClient) (string, error) {
			if statusMap, exists := tc.info["status_map"].(map[string]any); exists {
				for k, v := range statusMap {
					if k == targetName {
						if v == model.S_ALIVE.String() {
							tc.t.Errorf("Expected %s to be dead, but it is still alive", targetName)
						} else {
							tc.t.Logf("Agent %s is dead as expected", targetName)
						}
					} else {
						if v != model.S_ALIVE.String() {
							tc.t.Errorf("Expected %s to be alive, but it is dead", k)
						} else {
							tc.t.Logf("Agent %s is alive as expected", k)
						}
					}
				}
				return "", nil
			} else {
				tc.t.Error("status_map not found in info")
			}
			return "", nil
		},
	}
	ExecuteGame(t, config, handlers)
}
