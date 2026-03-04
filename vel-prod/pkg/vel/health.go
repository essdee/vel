package vel

// HealthCheck represents a single verify check that apps can register.
type HealthCheck struct {
	Name  string             // e.g. "sessions-data"
	Desc  string             // human-readable description
	Check func() (bool, string) // returns (pass, detail)
}

var healthChecks []HealthCheck

// RegisterCheck registers a health check. Call from init() in app server code.
func RegisterCheck(hc HealthCheck) {
	healthChecks = append(healthChecks, hc)
}

// GetHealthChecks returns all registered health checks.
func GetHealthChecks() []HealthCheck {
	return healthChecks
}
