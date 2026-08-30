package ipwhitelist

import "time"

// DefaultPath is the JSON file used when Open receives an empty path.
const DefaultPath = "./data/ipwhitelist.json"

// DefaultPeriod is how long an IP stays whitelisted after WhitelistTime (48h).
// This is the temporary whitelist lifetime, not the periodic disk-save interval.
const DefaultPeriod = 48 * time.Hour

// DefaultSaveInterval is the default ticker period for StartPeriodicSave when
// callers pass a non-positive interval.
const DefaultSaveInterval = 30 * time.Second

// Element is one temporary IP whitelist entry.
// WhitelistTime is when the IP was added; it remains active for DefaultPeriod.
type Element struct {
	IP            string    `json:"ip"`
	WhitelistTime time.Time `json:"whitelistTime"`
}

// ExpiresAt returns when this entry stops being whitelisted.
func (e Element) ExpiresAt(period time.Duration) time.Time {
	if period <= 0 {
		period = DefaultPeriod
	}
	return e.WhitelistTime.Add(period)
}

// Active reports whether the entry covers now within period after WhitelistTime.
func (e Element) Active(now time.Time, period time.Duration) bool {
	if e.IP == "" || e.WhitelistTime.IsZero() {
		return false
	}
	if period <= 0 {
		period = DefaultPeriod
	}
	return !now.Before(e.WhitelistTime) && now.Before(e.ExpiresAt(period))
}
