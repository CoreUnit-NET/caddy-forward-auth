package floodban

import "time"

// DefaultPath is the JSON file used when Open receives an empty path.
const DefaultPath = "./data/ban.json"

// DefaultSaveInterval is the default ticker period for StartPeriodicSave when
// callers pass a non-positive interval.
const DefaultSaveInterval = 30 * time.Second

// Ban is one temporary or permanent ban for a remote IP.
// When Permanent is true, ExpiresAt is ignored and the ban never auto-expires.
type Ban struct {
	IP        string    `json:"ip"`
	Permanent bool      `json:"permanent"`
	BannedAt  time.Time `json:"bannedAt"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	Rule      string    `json:"rule,omitempty"`
}

// Active reports whether this ban applies at now.
func (b Ban) Active(now time.Time) bool {
	if b.IP == "" {
		return false
	}
	if b.Permanent {
		return true
	}
	if b.BannedAt.IsZero() || b.ExpiresAt.IsZero() {
		return false
	}
	return !now.Before(b.BannedAt) && now.Before(b.ExpiresAt)
}

// HarsherThan reports whether candidate is a stronger punishment than current.
// Permanent always wins. Otherwise the later ExpiresAt wins. Equal strength
// returns false so callers can skip dirty writes.
func (b Ban) HarsherThan(current Ban) bool {
	if b.Permanent {
		return !current.Permanent
	}
	if current.Permanent {
		return false
	}
	return b.ExpiresAt.After(current.ExpiresAt)
}
