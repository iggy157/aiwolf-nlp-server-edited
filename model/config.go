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
		AgentCount    int `yaml:"agent_count"`
		CustomProfile struct {
			Enable         bool      `yaml:"enable"`
			Profiles       []Profile `yaml:"profiles"`
			DynamicProfile struct {
				Enable   bool     `yaml:"enable"`
				Prompt   string   `yaml:"prompt"`
				Attempts int      `yaml:"attempts"`
				Avatars  []string `yaml:"avatars"`
			} `yaml:"dynamic_profile"`
		} `yaml:"custom_profile"`
		VoteVisibility        bool    `yaml:"vote_visibility"`
		TalkOnFirstDay        bool    `yaml:"talk_on_first_day"`
		MaxContinueErrorRatio float64 `yaml:"max_continue_error_ratio"`
		Talk                  struct {
			TalkConfig `yaml:",inline"`
		} `yaml:"talk"`
		Whisper struct {
			TalkConfig `yaml:",inline"`
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
		Enable bool          `yaml:"enable"`
		Delay  time.Duration `yaml:"delay"`
	} `yaml:"realtime_broadcaster"`
	TTSBroadcaster struct {
		Enable         bool          `yaml:"enable"`
		Async          bool          `yaml:"async"`
		TargetDuration time.Duration `yaml:"target_duration"`
		SegmentDir     string        `yaml:"segment_dir"`
		TempDir        string        `yaml:"temp_dir"`
		Host           string        `yaml:"host"`
		Timeout        time.Duration `yaml:"timeout"`
		FfmpegPath     string        `yaml:"ffmpeg_path"`
		FfprobePath    string        `yaml:"ffprobe_path"`
		ConvertArgs    []string      `yaml:"convert_args"`
		DurationArgs   []string      `yaml:"duration_args"`
		PreConvertArgs []string      `yaml:"pre_convert_args"`
		SplitArgs      []string      `yaml:"split_args"`
	} `yaml:"tts_broadcaster"`
	Matching struct {
		SelfMatch    bool   `yaml:"self_match"`
		IsOptimize   bool   `yaml:"is_optimize"`
		TeamCount    int    `yaml:"team_count"`
		GameCount    int    `yaml:"game_count"`
		OutputPath   string `yaml:"output_path"`
		InfiniteLoop bool   `yaml:"infinite_loop"`
	} `yaml:"matching"`
}

type TalkConfig struct {
	MaxCount struct {
		PerAgent int `yaml:"per_agent"`
		PerDay   int `yaml:"per_day"`
	} `yaml:"max_count"`
	MaxLength struct {
		CountInWord   bool `yaml:"count_in_word"`
		PerTalk       int  `yaml:"per_talk"`
		MentionLength int  `yaml:"mention_length"`
		PerAgent      int  `yaml:"per_agent"`
		BaseLength    int  `yaml:"base_length"`
	} `yaml:"max_length"`
	MaxSkip int `yaml:"max_skip"`
}

type Profile struct {
	Name        string `yaml:"name"`
	AvatarURL   string `yaml:"avatar_url"`
	VoiceID     int    `yaml:"voice_id"`
	Age         int    `yaml:"age"`
	Gender      string `yaml:"gender"`
	Personality string `yaml:"personality"`
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
