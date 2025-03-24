package model

import "encoding/json"

type Info struct {
	GameID         string           `json:"gameID"`
	Day            int              `json:"day"`
	Agent          *Agent           `json:"agent"`
	MediumResult   *Judge           `json:"mediumResult,omitempty"`
	DivineResult   *Judge           `json:"divineResult,omitempty"`
	ExecutedAgent  *Agent           `json:"executedAgent,omitempty"`
	AttackedAgent  *Agent           `json:"attackedAgent,omitempty"`
	VoteList       []Vote           `json:"voteList,omitempty"`
	AttackVoteList []Vote           `json:"attackVoteList,omitempty"`
	TalkList       []Talk           `json:"-"`
	WhisperList    []Talk           `json:"-"`
	StatusMap      map[Agent]Status `json:"statusMap"`
	RoleMap        map[Agent]Role   `json:"roleMap"`
	RemainCount    *int             `json:"remainCount,omitempty"`
	RemainLength   *int             `json:"remainLength,omitempty"`
	RemainSkip     *int             `json:"remainSkip,omitempty"`
}

func (i Info) MarshalJSON() ([]byte, error) {
	statusMap := make(map[string]Status)
	for k, v := range i.StatusMap {
		statusMap[k.String()] = v
	}
	roleMap := make(map[string]Role)
	for k, v := range i.RoleMap {
		roleMap[k.String()] = v
	}
	type Alias Info
	return json.Marshal(&struct {
		*Alias
		StatusMap map[string]Status `json:"statusMap"`
		RoleMap   map[string]Role   `json:"roleMap"`
	}{
		Alias:     (*Alias)(&i),
		StatusMap: statusMap,
		RoleMap:   roleMap,
	})
}

func NewInfo(id string, agent *Agent, gameStatus *GameStatus, lastGameStatus *GameStatus, settings *Setting) Info {
	info := Info{
		GameID: id,
		Day:    gameStatus.Day,
		Agent:  agent,
	}
	if lastGameStatus != nil {
		if lastGameStatus.MediumResult != nil && agent.Role == R_MEDIUM {
			info.MediumResult = lastGameStatus.MediumResult
		}
		if lastGameStatus.DivineResult != nil && agent.Role == R_SEER {
			info.DivineResult = lastGameStatus.DivineResult
		}
		if lastGameStatus.ExecutedAgent != nil {
			info.ExecutedAgent = lastGameStatus.ExecutedAgent
		}
		if lastGameStatus.AttackedAgent != nil {
			info.AttackedAgent = lastGameStatus.AttackedAgent
		}
		if settings.VoteVisibility {
			info.VoteList = lastGameStatus.Votes
		}
		if settings.VoteVisibility && agent.Role == R_WEREWOLF {
			info.AttackVoteList = lastGameStatus.AttackVotes
		}
	}
	info.TalkList = gameStatus.Talks
	if agent.Role == R_WEREWOLF {
		info.WhisperList = gameStatus.Whispers
	}
	info.StatusMap = gameStatus.StatusMap
	roleMap := make(map[Agent]Role)
	roleMap[*agent] = agent.Role
	if agent.Role == R_WEREWOLF {
		for a := range gameStatus.StatusMap {
			if a.Role == R_WEREWOLF {
				roleMap[a] = a.Role
			}
		}
	}
	info.RoleMap = roleMap
	if gameStatus.RemainCountMap != nil {
		count := (*gameStatus.RemainCountMap)[*agent]
		info.RemainCount = &count
	}
	if gameStatus.RemainLengthMap != nil {
		if value, exists := (*gameStatus.RemainLengthMap)[*agent]; exists {
			info.RemainLength = &value
		}
	}
	if gameStatus.RemainSkipMap != nil {
		count := (*gameStatus.RemainSkipMap)[*agent]
		info.RemainSkip = &count
	}
	return info
}
