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
	Symbol    string
	Bid       float64
	Ask       float64
	EventTime time.Time
	Received  time.Time
}

type Candidate struct {
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
	ID             int64    `json:"id"`
	RuleID         int64    `json:"ruleId"`
	RuleName       string   `json:"ruleName"`
	Symbol         string   `json:"symbol"`
	Direction      string   `json:"direction"`
	ChangePercent  float64  `json:"changePercent"`
	Price          float64  `json:"price"`
	SpreadPercent  float64  `json:"spreadPercent"`
	QuoteVolume    float64  `json:"quoteVolume"`
	EventTimeMs    int64    `json:"eventTimeMs"`
	CreatedAtMs    int64    `json:"createdAtMs"`
	Sound          string   `json:"sound"`
	Color          string   `json:"color"`
	Volume         float64  `json:"volume"`
	Terminals      []string `json:"terminals"`
	TerminalStatus string   `json:"terminalStatus"`
	TerminalError  string   `json:"terminalError"`
}

type Health struct {
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
