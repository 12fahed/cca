package transcript

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
