package wsmsg

type Message struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Delta   string `json:"delta,omitempty"`
}
