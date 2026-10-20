package core

import "time"

type Rule struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Percent        float64  `json:"percent"`
	WindowSeconds  int      `json:"windowSeconds"`
	Sound          string   `json:"sound"`
	Color          string   `json:"color"`
	Volume         float64  `json:"volume"`
	MinQuoteVolume float64  `json:"minQuoteVolume"`
	Terminals      []string `json:"terminals"`
}

type Settings struct {
	Language        string         `json:"language"`
	Proxies         []ProxyProfile `json:"proxies"`
	ActiveProxyID   string         `json:"activeProxyId"`
	CooldownSeconds int            `json:"cooldownSeconds"`
	MonitorID       string         `json:"monitorId"`
	TigerPort       int            `json:"tigerPort"`
	TigerLinkGroup  string         `json:"tigerLinkGroup"`
}

func DefaultSettings() Settings {
	return Settings{Proxies: []ProxyProfile{}, Language: "ru", CooldownSeconds: 60, TigerPort: 7819, TigerLinkGroup: "A"}
}

type Quote struct {
	UpdateID    uint64
	ClockOffset time.Duration
	Symbol      string
	Bid         float64
	Ask         float64
	EventTime   time.Time
	Received    time.Time
}

type Candidate struct {
	Detected      time.Time
	Expires       time.Time
	Rule          Rule
	Symbol        string
	Direction     string
	ChangePercent float64
	Price         float64
	SpreadPercent float64
	QuoteVolume   float64
	EventTime     time.Time
	Received      time.Time
}

type Alert struct {
	ReceivedAtMs     int64    `json:"receivedAtMs"`
	DetectedAtMs     int64    `json:"detectedAtMs"`
	WriteStartedAtMs int64    `json:"writeStartedAtMs"`
	ID               int64    `json:"id"`
	RuleID           int64    `json:"ruleId"`
	RuleName         string   `json:"ruleName"`
	Symbol           string   `json:"symbol"`
	Direction        string   `json:"direction"`
	ChangePercent    float64  `json:"changePercent"`
	Price            float64  `json:"price"`
	SpreadPercent    float64  `json:"spreadPercent"`
	QuoteVolume      float64  `json:"quoteVolume"`
	EventTimeMs      int64    `json:"eventTimeMs"`
	CreatedAtMs      int64    `json:"createdAtMs"`
	Sound            string   `json:"sound"`
	Color            string   `json:"color"`
	Volume           float64  `json:"volume"`
	Terminals        []string `json:"terminals"`
	TerminalStatus   string   `json:"terminalStatus"`
	TerminalError    string   `json:"terminalError"`
}

type StreamHealth struct {
	AgeP99Ms      int64  `json:"ageP99Ms"`
	AgeMaxMs      int64  `json:"ageMaxMs"`
	Shard         int    `json:"shard"`
	State         string `json:"state"`
	Subscriptions int    `json:"subscriptions"`
	QuoteAgeMs    *int64 `json:"quoteAgeMs"`
	Detail        string `json:"detail"`
}

type QueueHealth struct {
	Size     int    `json:"size"`
	OldestMs int64  `json:"oldestMs"`
	WriteMs  int64  `json:"writeMs"`
	Unsaved  uint64 `json:"unsaved"`
	Error    string `json:"error"`
}

type AlertUpdate struct {
	ID             int64  `json:"id"`
	TerminalStatus string `json:"terminalStatus"`
	TerminalError  string `json:"terminalError"`
}

type AlertTiming struct {
	ID                  int64 `json:"id"`
	ReceivedAtMs        int64 `json:"receivedAtMs"`
	DetectedAtMs        int64 `json:"detectedAtMs"`
	CommittedAtMs       int64 `json:"committedAtMs"`
	TerminalSentAtMs    int64 `json:"terminalSentAtMs"`
	TerminalDoneAtMs    int64 `json:"terminalDoneAtMs"`
	FeedRenderedAtMs    int64 `json:"feedRenderedAtMs"`
	OverlayRenderedAtMs int64 `json:"overlayRenderedAtMs"`
}

type Health struct {
	Timings           []AlertTiming  `json:"timings"`
	QuoteAgeMs        *int64         `json:"quoteAgeMs"`
	Streams           []StreamHealth `json:"streams"`
	Storage           QueueHealth    `json:"storage"`
	VolumeAgeMs       int64          `json:"volumeAgeMs"`
	ServiceError      string         `json:"serviceError"`
	UsedWeight        int64          `json:"usedWeight"`
	TerminalQueued    int            `json:"terminalQueued"`
	TerminalErrors    uint64         `json:"terminalErrors"`
	RenderDelayMs     int64          `json:"renderDelayMs"`
	TerminalWaitMs    int64          `json:"terminalWaitMs"`
	TerminalRequestMs int64          `json:"terminalRequestMs"`

	ViaProxy    bool   `json:"viaProxy"`
	PingMs      *int64 `json:"pingMs"`
	State       string `json:"state"`
	Detail      string `json:"detail"`
	Connections int    `json:"connections"`
	Symbols     int    `json:"symbols"`
	Ready       int    `json:"ready"`
	UpdatedAtMs int64  `json:"updatedAtMs"`
}

type Snapshot struct {
	Rules    []Rule   `json:"rules"`
	Settings Settings `json:"settings"`
	Health   Health   `json:"health"`
	Alerts   []Alert  `json:"alerts"`
}
