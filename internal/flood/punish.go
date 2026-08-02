package flood

import (
	"time"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/floodban"
)

// UpdatePunishments inspects flood tracking counts for ip and upserts the
// harshest matching ban rule into the ban bundle.
func (e *Engine) UpdatePunishments(ip string, now time.Time) {
	if e == nil || e.Track == nil || e.Bans == nil || ip == "" {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var best floodban.Ban
	for _, rule := range rules {
		count := e.Track.CountSince(ip, now.Add(-rule.Window))
		if count < rule.Count {
			continue
		}
		candidate := banFromRule(ip, now, rule)
		if best.IP == "" || candidate.HarsherThan(best) {
			best = candidate
		}
	}
	if best.IP == "" {
		return
	}
	e.Bans.Upsert(best)
}
