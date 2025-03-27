package model

import "encoding/json"

type Info struct {
	GameID         string           `json:"gameID"`
	Day            int              `json:"day"`
	Agent          *Agent           `json:"agent"`
	Profile        *string          `json:"profile,omitempty"`
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
