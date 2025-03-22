package model

type GameStatus struct {
	Day           int
	MediumResult  *Judge
	DivineResult  *Judge
	ExecutedAgent *Agent
	AttackedAgent *Agent
	Guard         *Guard
	Votes         []Vote
	AttackVotes   []Vote
	Talks         []Talk
	Whispers      []Talk
	StatusMap     map[Agent]Status
}

func NewInitializeGameStatus(agents []*Agent) GameStatus {
	status := GameStatus{
		Day:           0,
		MediumResult:  nil,
		DivineResult:  nil,
		ExecutedAgent: nil,
		AttackedAgent: nil,
		Guard:         nil,
		Votes:         []Vote{},
		AttackVotes:   []Vote{},
		Talks:         []Talk{},
		Whispers:      []Talk{},
		StatusMap:     make(map[Agent]Status),
	}
	for _, agent := range agents {
		status.StatusMap[*agent] = S_ALIVE
	}
	return status
}

func (g GameStatus) NextDay() GameStatus {
	status := GameStatus{
		Day:           g.Day + 1,
		MediumResult:  nil,
		DivineResult:  nil,
		ExecutedAgent: nil,
		AttackedAgent: nil,
		Guard:         nil,
		Votes:         []Vote{},
		AttackVotes:   []Vote{},
		Talks:         []Talk{},
		Whispers:      []Talk{},
		StatusMap:     make(map[Agent]Status),
	}
	for agent, s := range g.StatusMap {
		status.StatusMap[agent] = s
	}
	return status
}
