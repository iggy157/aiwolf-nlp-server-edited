package model

type Packet struct {
	Request        *Request `json:"request"`
	Info           *Info    `json:"info,omitempty"`
	Setting        *Setting `json:"setting,omitempty"`
	TalkHistory    *[]Talk  `json:"talk_history,omitempty"`
	WhisperHistory *[]Talk  `json:"whisper_history,omitempty"`
}
