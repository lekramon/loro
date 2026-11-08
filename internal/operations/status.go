package operations

// Status is deliberately limited to safe operational state. It never exposes
// archive paths, credentials, database errors, or private filesystem details.
type Status struct {
	State       string `json:"state"`
	Maintenance string `json:"maintenance"`
}
