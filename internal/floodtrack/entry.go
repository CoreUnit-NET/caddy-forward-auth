package floodtrack

import "time"

// DefaultPath is the JSON file used when Open receives an empty path.
const DefaultPath = "./data/flood.json"

// DefaultRetention is how long flood entries are kept (168h).
// Cleanup outside this package may use this as the cutoff window.
const DefaultRetention = 168 * time.Hour

// DefaultSaveInterval is the default ticker period for StartPeriodicSave when
// callers pass a non-positive interval.
const DefaultSaveInterval = 30 * time.Second

// Entry is one failed-auth flood tracking event.
// It stores when the failure happened, the remote IP, and the service name
// (may be empty when the service is unknown).
type Entry struct {
	Time    time.Time `json:"time"`
	IP      string    `json:"ip"`
	Service string    `json:"service"`
}
