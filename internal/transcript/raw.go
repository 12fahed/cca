package transcript

import "encoding/json"

// The transcript format is internal to Claude Code and undocumented. These
// types mirror only the fields cca needs; every one is optional in practice, so
// pointers mark presence where absence and zero mean different things.

type rawRecord struct {
	Type        string      `json:"type"`
	UUID        string      `json:"uuid"`
	SessionID   string      `json:"sessionId"`
	RequestID   string      `json:"requestId"`
	Timestamp   string      `json:"timestamp"`
	CWD         string      `json:"cwd"`
	IsSidechain bool        `json:"isSidechain"`
	Message     *rawMessage `json:"message"`
}

type rawMessage struct {
	ID    string    `json:"id"`
	Model string    `json:"model"`
	Usage *rawUsage `json:"usage"`
}

type rawUsage struct {
	InputTokens              int64             `json:"input_tokens"`
	OutputTokens             int64             `json:"output_tokens"`
	CacheCreationInputTokens int64             `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64             `json:"cache_read_input_tokens"`
	CacheCreation            *rawCacheCreation `json:"cache_creation"`
	OutputTokensDetails      *rawOutputDetails `json:"output_tokens_details"`
	ServerToolUse            *rawServerToolUse `json:"server_tool_use"`
	ServiceTier              *string           `json:"service_tier"`
	InferenceGeo             *string           `json:"inference_geo"`
	Speed                    *string           `json:"speed"`
}

type rawCacheCreation struct {
	Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
	Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
}

type rawOutputDetails struct {
	ThinkingTokens int64 `json:"thinking_tokens"`
}

type rawServerToolUse struct {
	WebSearchRequests int64 `json:"web_search_requests"`
	WebFetchRequests  int64 `json:"web_fetch_requests"`
}

// cost-state records carry Claude Code's own cost accounting for a session.
// cca never sums them: they are periodic snapshots, so adding them would
// multiply a session's spend by however often it happened to be written. They
// exist here as an independent oracle to check cca's arithmetic against.

type rawCostState struct {
	Type         string                     `json:"type"`
	SessionID    string                     `json:"sessionId"`
	TotalCostUSD float64                    `json:"totalCostUSD"`
	ModelUsage   map[string]rawCostStateUse `json:"modelUsage"`
	HasUnknown   bool                       `json:"hasUnknownModelCost"`
}

type rawCostStateUse struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	ThinkingTokens           int64   `json:"thinkingTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	WebSearchRequests        int64   `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
}

// Title records are appended inline to a session's transcript. The two kinds
// carry their text under different keys, and the key order within a record
// varies between them, so both are decoded by field name rather than position.

type rawTitle struct {
	Type        string `json:"type"`
	SessionID   string `json:"sessionId"`
	CustomTitle string `json:"customTitle"`
	AITitle     string `json:"aiTitle"`
}

// rawUserRecord is a user turn, read only for the last-resort title fallback.
type rawUserRecord struct {
	SessionID   string          `json:"sessionId"`
	IsSidechain bool            `json:"isSidechain"`
	Message     *rawUserMessage `json:"message"`
}

type rawUserMessage struct {
	Content userContent `json:"content"`
}

// userContent is either a bare string or a list of blocks, depending on how the
// turn was produced. A single type that accepts both keeps the caller from
// having to know which.
type userContent struct {
	Text   string
	Blocks []rawContentBlock
}

type rawContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *userContent) UnmarshalJSON(data []byte) error {
	// Order matters: a JSON string must not be offered to the slice decoder.
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &c.Text)
	}
	if len(data) > 0 && data[0] == '[' {
		return json.Unmarshal(data, &c.Blocks)
	}
	// Anything else (null, an object) carries no prompt text; not an error.
	return nil
}
