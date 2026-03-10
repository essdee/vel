package vel

import "sync"

var testModeMu sync.RWMutex
var (
	testModeEnabled bool
	fixtureName     string
)

// SetTestMode enables or disables test mode with the given fixture name.
func SetTestMode(enabled bool, fixture string) {
	testModeMu.Lock()
	defer testModeMu.Unlock()
	testModeEnabled = enabled
	fixtureName = fixture
}

// IsTestMode reports whether the server is running in test mode.
func IsTestMode() bool {
	testModeMu.RLock()
	defer testModeMu.RUnlock()
	return testModeEnabled
}

// FixtureName returns the active fixture set name (e.g. "default", "empty", "demo").
func FixtureName() string {
	testModeMu.RLock()
	defer testModeMu.RUnlock()
	return fixtureName
}
