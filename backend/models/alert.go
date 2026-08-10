package models

import (
	"time"
)

// AlertSeverity ranks how much attention an alert deserves. The console orders
// the feed by this, so the constants double as the sort key via SeverityRank.
type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical"
	AlertWarning  AlertSeverity = "warning"
	AlertNotice   AlertSeverity = "notice"
	AlertInfo     AlertSeverity = "info"
)

// SeverityRank orders severities most-urgent-first. Unknown values sort last.
func SeverityRank(s AlertSeverity) int {
	switch s {
	case AlertCritical:
		return 0
	case AlertWarning:
		return 1
	case AlertNotice:
		return 2
	case AlertInfo:
		return 3
	default:
		return 4
	}
}

// AlertState is the lifecycle of an alert. Rules re-emit the same alert on every
// evaluation; the state is what the operator controls.
//
//	open     — the condition holds and nobody has looked at it
//	acked    — an operator acknowledged it; re-emitting does NOT reopen it
//	resolved — the condition stopped holding on a later evaluation
type AlertState string

const (
	AlertStateOpen     AlertState = "open"
	AlertStateAcked    AlertState = "acked"
	AlertStateResolved AlertState = "resolved"
)

// Alert is one finding produced by a rule in internal/alert.
//
// DedupeKey is what makes re-evaluation idempotent: rules run after every sweep
// (every 30s) and emit the full current set of findings, so the repository
// upserts on this key rather than inserting duplicates.
type Alert struct {
	ID          string        `json:"id"`
	RuleID      string        `json:"rule_id"`
	DedupeKey   string        `json:"-"`
	Severity    AlertSeverity `json:"severity"`
	Title       string        `json:"title"`
	Detail      string        `json:"detail"`
	DeviceID    *string       `json:"device_id,omitempty"`
	NetworkID   *string       `json:"network_id,omitempty"`
	State       AlertState    `json:"state"`
	Occurrences int           `json:"occurrences"`
	FirstSeenAt time.Time     `json:"first_seen_at"`
	LastSeenAt  time.Time     `json:"last_seen_at"`
	AckedAt     *time.Time    `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// AlertQuery filters a FindAll. Zero values mean "no filter".
type AlertQuery struct {
	States     []AlertState
	Severities []AlertSeverity
	DeviceID   string
	Limit      int
}

// AlertCounts is the per-severity breakdown the console's header tile renders.
type AlertCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Notice   int `json:"notice"`
	Info     int `json:"info"`
}

// Total is the number of alerts across all severities.
func (c AlertCounts) Total() int {
	return c.Critical + c.Warning + c.Notice + c.Info
}
