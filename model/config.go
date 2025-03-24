package model

import (
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		WebSocket struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"web_socket"`
		Authentication struct {
			Enable bool   `yaml:"enable"`
			Secret string `yaml:"secret"`
		} `yaml:"authentication"`
	} `yaml:"server"`
	Game struct {
		AgentCount            int     `yaml:"agent_count"`
		VoteVisibility        bool    `yaml:"vote_visibility"`
		TalkOnFirstDay        bool    `yaml:"talk_on_first_day"`
		MaxContinueErrorRatio float64 `yaml:"max_continue_error_ratio"`
		Talk                  struct {
			MaxCount struct {
				PerAgent int `yaml:"per_agent"`
				PerDay   int `yaml:"per_day"`
			} `yaml:"max_count"`
			MaxLength struct {
				PerTalk    int `yaml:"per_talk"`
				PerAgent   int `yaml:"per_agent"`
				BaseLength int `yaml:"base_length"`
			} `yaml:"max_length"`
			MaxSkip int `yaml:"max_skip"`
		} `yaml:"talk"`
		Whisper struct {
			MaxCount struct {
				PerAgent int `yaml:"per_agent"`
				PerDay   int `yaml:"per_day"`
			} `yaml:"max_count"`
			MaxLength struct {
				PerTalk    int `yaml:"per_talk"`
				PerAgent   int `yaml:"per_agent"`
				BaseLength int `yaml:"base_length"`
			} `yaml:"max_length"`
			MaxSkip int `yaml:"max_skip"`
		} `yaml:"whisper"`
		Vote struct {
			MaxCount int `yaml:"max_count"`
		} `yaml:"vote"`
		AttackVote struct {
			MaxCount      int  `yaml:"max_count"`
			AllowNoTarget bool `yaml:"allow_no_target"`
		} `yaml:"attack_vote"`
		Timeout struct {
			Action     time.Duration `yaml:"action"`
			Response   time.Duration `yaml:"response"`
			Acceptable time.Duration `yaml:"acceptable"`
		} `yaml:"timeout"`
	} `yaml:"game"`
	JSONLogger struct {
		Enable    bool   `yaml:"enable"`
		OutputDir string `yaml:"output_dir"`
		Filename  string `yaml:"filename"`
	} `yaml:"json_logger"`
	GameLogger struct {
		Enable    bool   `yaml:"enable"`
		OutputDir string `yaml:"output_dir"`
		Filename  string `yaml:"filename"`
	} `yaml:"game_logger"`
	RealtimeBroadcaster struct {
		Enable bool `yaml:"enable"`
	} `yaml:"realtime_broadcaster"`
	Matching struct {
		SelfMatch    bool   `yaml:"self_match"`
		IsOptimize   bool   `yaml:"is_optimize"`
		TeamCount    int    `yaml:"team_count"`
		GameCount    int    `yaml:"game_count"`
		OutputPath   string `yaml:"output_path"`
		InfiniteLoop bool   `yaml:"infinite_loop"`
	} `yaml:"matching"`
}

func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("設定ファイルの読み込みに失敗しました", "error", err)
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		slog.Error("設定ファイルのパースに失敗しました", "error", err)
		return nil, err
	}
	return &config, nil
}
