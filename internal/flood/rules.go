package flood

import (
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodban"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/settings"
)

// punishmentRule is kept as an alias for internal rule storage on Engine.
type punishmentRule = Rule

type defaultTier struct {
	count, windowMins, banMins int
	permanent                  bool
}

func defaultRules() []Rule {
	defs := []defaultTier{
		{10, 2, 3, false},
		{60, 30, 120, false},
		{90, 60, 0, true},
		{120, 360, 0, true},
		{240, 10080, 0, true},
	}
	out := make([]Rule, 0, len(defs))
	for _, d := range defs {
		rule := Rule{
			ID:        settings.FormatTierID(d.count, d.windowMins),
			Count:     d.count,
			Window:    time.Duration(d.windowMins) * time.Minute,
			Permanent: d.permanent,
		}
		if !d.permanent {
			rule.TempDuration = time.Duration(d.banMins) * time.Minute
		}
		out = append(out, rule)
	}
	return out
}

func banFromRule(ip string, now time.Time, rule Rule) floodban.Ban {
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
