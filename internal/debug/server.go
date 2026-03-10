package debug

import (
	"fmt"
	"net/http"
)

// StartDebugServer starts the debug HTTP server on the configured port.
// It runs in a goroutine and never blocks. Only binds to localhost.
func StartDebugServer(cfg DebugConfig) {
	if !cfg.Enabled {
		return
	}

	mux := http.NewServeMux()
	RegisterEndpoints(mux, cfg)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.DebugPort)
	fmt.Printf("[Debug] Debug server listening on %s\n", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Printf("[Debug] Debug server error: %v\n", err)
		}
	}()
}
