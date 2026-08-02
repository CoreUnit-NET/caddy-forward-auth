package flood

import (
	"time"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/floodtrack"
)

// RecordFailure appends a failed-auth event and updates punishments for ip.
// It does not write bundles to disk (dirty + periodic save owns persistence).
func (e *Engine) RecordFailure(ip, service string, now time.Time) {
	if e == nil || e.Track == nil || ip == "" {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	e.Track.Append(floodtrack.Entry{
		Time:    now,
		IP:      ip,
		Service: service,
	})
	e.UpdatePunishments(ip, now)
}
