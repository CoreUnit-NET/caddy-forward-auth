package flood

import (
	"time"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/floodban"
)

// punishmentRule is one flood threshold that may introduce a ban.
type punishmentRule struct {
	ID           string
	Count        int
	Window       time.Duration
	TempDuration time.Duration // ignored when Permanent
	Permanent    bool
}

// rules are evaluated on each failure; the harshest matching rule wins.
var rules = []punishmentRule{
	{ID: "10/2m", Count: 10, Window: 2 * time.Minute, TempDuration: 3 * time.Minute},
	{ID: "60/30m", Count: 60, Window: 30 * time.Minute, TempDuration: 2 * time.Hour},
	{ID: "90/60m", Count: 90, Window: 60 * time.Minute, Permanent: true},
	{ID: "120/6h", Count: 120, Window: 6 * time.Hour, Permanent: true},
	{ID: "240/168h", Count: 240, Window: 168 * time.Hour, Permanent: true},
}

// harsherBan reports whether a is a stronger punishment than b.
func harsherBan(a, b floodban.Ban) bool {
	if a.IP == "" {
		return false
	}
	if b.IP == "" {
		return true
	}
	if a.Permanent {
		return !b.Permanent
	}
	if b.Permanent {
		return false
	}
	return a.ExpiresAt.After(b.ExpiresAt)
}

func banFromRule(ip string, now time.Time, rule punishmentRule) floodban.Ban {
	ban := floodban.Ban{
		IP:        ip,
		BannedAt:  now,
		Permanent: rule.Permanent,
		Rule:      rule.ID,
	}
	if !rule.Permanent {
		ban.ExpiresAt = now.Add(rule.TempDuration)
	}
	return ban
}
