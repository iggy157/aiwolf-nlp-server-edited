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
