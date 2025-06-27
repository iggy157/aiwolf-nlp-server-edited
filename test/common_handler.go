package test

import (
	"errors"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

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
