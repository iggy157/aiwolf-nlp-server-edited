package model

import (
	"encoding/json"
	"errors"
)

type Setting struct {
	AgentCount     int          `json:"agentCount"`
	RoleNumMap     map[Role]int `json:"roleNumMap"`
	VoteVisibility bool         `json:"voteVisibility"`
	TalkOnFirstDay bool         `json:"talkOnFirstDay"`
	Talk           struct {
		MaxCount struct {
			PerAgent int `json:"perAgent"`
			PerDay   int `json:"perDay"`
		} `json:"maxCount"`
		MaxLength struct {
			PerTalk    *int `json:"perTalk,omitempty"`
			PerAgent   *int `json:"perAgent,omitempty"`
			BaseLength *int `json:"baseLength,omitempty"`
		} `json:"maxLength"`
		MaxSkip int `json:"maxSkip"`
	} `json:"talk"`
	Whisper struct {
		MaxCount struct {
			PerAgent int `json:"perAgent"`
			PerDay   int `json:"perDay"`
		} `json:"maxCount"`
		MaxLength struct {
			PerTalk    *int `json:"perTalk,omitempty"`
			PerAgent   *int `json:"perAgent,omitempty"`
			BaseLength *int `json:"baseLength,omitempty"`
		} `json:"maxLength"`
		MaxSkip int `json:"maxSkip"`
	} `json:"whisper"`
	Vote struct {
		MaxCount int `json:"maxCount"`
	} `json:"vote"`
	AttackVote struct {
		MaxCount      int  `json:"maxCount"`
		AllowNoTarget bool `json:"allowNoTarget"`
	} `json:"attackVote"`
	Timeout struct {
		Action   int `json:"action"`
		Response int `json:"response"`
	} `json:"timeout"`
}

func NewSetting(config Config) (*Setting, error) {
	roleNumMap := Roles(config.Game.AgentCount)
	if roleNumMap == nil {
		return nil, errors.New("対応する役職の人数がありません")
	}
	setting := Setting{
		AgentCount:     config.Game.AgentCount,
		RoleNumMap:     roleNumMap,
		VoteVisibility: config.Game.VoteVisibility,
		TalkOnFirstDay: config.Game.TalkOnFirstDay,
		Talk: struct {
			MaxCount struct {
				PerAgent int `json:"perAgent"`
				PerDay   int `json:"perDay"`
			} `json:"maxCount"`
			MaxLength struct {
				PerTalk    *int `json:"perTalk,omitempty"`
				PerAgent   *int `json:"perAgent,omitempty"`
				BaseLength *int `json:"baseLength,omitempty"`
			} `json:"maxLength"`
			MaxSkip int `json:"maxSkip"`
		}{
			MaxCount: struct {
				PerAgent int `json:"perAgent"`
				PerDay   int `json:"perDay"`
			}{
				PerAgent: config.Game.Talk.MaxCount.PerAgent,
				PerDay:   config.Game.Talk.MaxCount.PerDay,
			},
			MaxLength: struct {
				PerTalk    *int `json:"perTalk,omitempty"`
				PerAgent   *int `json:"perAgent,omitempty"`
				BaseLength *int `json:"baseLength,omitempty"`
			}{},
			MaxSkip: config.Game.Talk.MaxSkip,
		},
		Whisper: struct {
			MaxCount struct {
				PerAgent int `json:"perAgent"`
				PerDay   int `json:"perDay"`
			} `json:"maxCount"`
			MaxLength struct {
				PerTalk    *int `json:"perTalk,omitempty"`
				PerAgent   *int `json:"perAgent,omitempty"`
				BaseLength *int `json:"baseLength,omitempty"`
			} `json:"maxLength"`
			MaxSkip int `json:"maxSkip"`
		}{
			MaxCount: struct {
				PerAgent int `json:"perAgent"`
				PerDay   int `json:"perDay"`
			}{
				PerAgent: config.Game.Whisper.MaxCount.PerAgent,
				PerDay:   config.Game.Whisper.MaxCount.PerDay,
			},
			MaxLength: struct {
				PerTalk    *int `json:"perTalk,omitempty"`
				PerAgent   *int `json:"perAgent,omitempty"`
				BaseLength *int `json:"baseLength,omitempty"`
			}{},
			MaxSkip: config.Game.Whisper.MaxSkip,
		},
		Vote: struct {
			MaxCount int `json:"maxCount"`
		}{
			MaxCount: config.Game.Vote.MaxCount,
		},
		AttackVote: struct {
			MaxCount      int  `json:"maxCount"`
			AllowNoTarget bool `json:"allowNoTarget"`
		}{
			MaxCount:      config.Game.AttackVote.MaxCount,
			AllowNoTarget: config.Game.AttackVote.AllowNoTarget,
		},
		Timeout: struct {
			Action   int `json:"action"`
			Response int `json:"response"`
		}{
			Action:   int(config.Game.Timeout.Action.Milliseconds()),
			Response: int(config.Game.Timeout.Response.Milliseconds()),
		},
	}
	if config.Game.Talk.MaxLength.PerTalk != -1 {
		setting.Talk.MaxLength.PerTalk = &config.Game.Talk.MaxLength.PerTalk
	}
	if config.Game.Talk.MaxLength.PerAgent != -1 {
		setting.Talk.MaxLength.PerAgent = &config.Game.Talk.MaxLength.PerAgent
		setting.Talk.MaxLength.BaseLength = &config.Game.Talk.MaxLength.BaseLength
	}
	if config.Game.Whisper.MaxLength.PerTalk != -1 {
		setting.Whisper.MaxLength.PerTalk = &config.Game.Whisper.MaxLength.PerTalk
	}
	if config.Game.Whisper.MaxLength.PerAgent != -1 {
		setting.Whisper.MaxLength.PerAgent = &config.Game.Whisper.MaxLength.PerAgent
		setting.Whisper.MaxLength.BaseLength = &config.Game.Whisper.MaxLength.BaseLength
	}
	return &setting, nil
}

func (s Setting) MarshalJSON() ([]byte, error) {
	roleNumMap := make(map[string]int)
	for k, v := range s.RoleNumMap {
		roleNumMap[k.String()] = v
	}
	type Alias Setting
	return json.Marshal(&struct {
		*Alias
		RoleNumMap map[string]int `json:"roleNumMap"`
	}{
		Alias:      (*Alias)(&s),
		RoleNumMap: roleNumMap,
	})
}
