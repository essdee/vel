package vel

import "net/http"

// AppConfig is passed to app Register functions with runtime context.
type AppConfig struct {
	Name      string
	Dir       string
	Workspace string
}

// AppRegistration is how apps register their server-side routes.
// Apps call RegisterApp from init().
type AppRegistration struct {
	Name     string
	Register func(mux *http.ServeMux, cfg AppConfig)
}

var registrations []AppRegistration

// RegisterApp registers an app's server-side routes.
// Call this from init() in your app's server package.
func RegisterApp(reg AppRegistration) {
	registrations = append(registrations, reg)
}

// GetRegistrations returns all registered app registrations.
func GetRegistrations() []AppRegistration {
	return registrations
}
