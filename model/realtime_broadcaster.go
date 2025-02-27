package model

type BroadcastPacket struct {
	Id     string `json:"id"`
	Idx    int    `json:"idx"`
	Day    int    `json:"day"`
	IsDay  bool   `json:"isDay"`
	Agents []struct {
		Idx        int    `json:"idx"`
		Team       string `json:"team"`
		Name       string `json:"name"`
		Role       string `json:"role"`
		IsAlive    bool   `json:"isAlive"`
		TargetIdxs []int  `json:"targetIdxs"`
		IsBubble   bool   `json:"isBubble"`
	} `json:"agents"`
	Message   string `json:"message"`
	Summary   string `json:"summary"`
	IsDivider bool   `json:"isDivider"`
}
