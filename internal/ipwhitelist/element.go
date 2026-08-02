package ipwhitelist

import "time"

// DefaultPath is the JSON file used when Open receives an empty path.
const DefaultPath = "./data/ipwhitelist.json"

// DefaultPeriod is how long an IP stays whitelisted after WhitelistTime (48h).
// This is the temporary whitelist lifetime, not the periodic disk-save interval.
const DefaultPeriod = 48 * time.Hour

// Element is one temporary IP whitelist entry.
// WhitelistTime is when the IP was added; it remains active for DefaultPeriod.
type Element struct {
	IP            string    `json:"ip"`
	WhitelistTime time.Time `json:"whitelistTime"`
}

// ExpiresAt returns when this entry stops being whitelisted.
func (e Element) ExpiresAt() time.Time {
	return e.WhitelistTime.Add(DefaultPeriod)
}

// Active reports whether the entry covers now within DefaultPeriod.
func (e Element) Active(now time.Time) bool {
	if e.IP == "" || e.WhitelistTime.IsZero() {
		return false
	}
	return !now.Before(e.WhitelistTime) && now.Before(e.ExpiresAt())
}
