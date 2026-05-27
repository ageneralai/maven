package responses

type ResponseRef struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type OutputItem struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type CreatedEvent struct {
	Type     string      `json:"type"`
	Response ResponseRef `json:"response"`
}

type OutputItemAddedEvent struct {
	Type  string     `json:"type"`
	Index int        `json:"index"`
	Item  OutputItem `json:"item"`
}

type ContentPartAddedEvent struct {
	Type         string `json:"type"`
	ItemIndex    int    `json:"item_index"`
	ContentIndex int    `json:"content_index"`
}

type OutputTextDeltaEvent struct {
	Type         string `json:"type"`
	ItemIndex    int    `json:"item_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type ItemIndexEvent struct {
	Type         string `json:"type"`
	ItemIndex    int    `json:"item_index"`
	ContentIndex int    `json:"content_index,omitempty"`
	Index        int    `json:"index,omitempty"`
}

type CompletedEvent struct {
	Type     string      `json:"type"`
	Response ResponseRef `json:"response"`
}

type Request struct {
	Input              any    `json:"input"`
	PreviousResponseID string `json:"previous_response_id"`
}

type ErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
