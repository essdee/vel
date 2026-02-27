package hooks

import "sync"

// FilterFunc transforms data and returns the modified version.
type FilterFunc func(data interface{}, ctx interface{}) interface{}

// ActionFunc is called when an event is emitted.
type ActionFunc func(args ...interface{})

// Engine provides a WordPress-style hook system for Go plugins.
type Engine struct {
	mu      sync.RWMutex
	filters map[string][]FilterFunc
	actions map[string][]ActionFunc
}

// New creates a new hook engine.
func New() *Engine {
	return &Engine{
		filters: make(map[string][]FilterFunc),
		actions: make(map[string][]ActionFunc),
	}
}

// AddFilter registers a filter function for the given hook name.
func (e *Engine) AddFilter(name string, fn FilterFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filters[name] = append(e.filters[name], fn)
}

// Filter runs all registered filters for the hook, piping data through each.
func (e *Engine) Filter(name string, data interface{}, ctx interface{}) interface{} {
	e.mu.RLock()
	fns := e.filters[name]
	e.mu.RUnlock()
	for _, fn := range fns {
		data = fn(data, ctx)
	}
	return data
}

// On registers an action handler for the given event name.
func (e *Engine) On(name string, fn ActionFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.actions[name] = append(e.actions[name], fn)
}

// Emit fires all action handlers registered for the event.
func (e *Engine) Emit(name string, args ...interface{}) {
	e.mu.RLock()
	fns := e.actions[name]
	e.mu.RUnlock()
	for _, fn := range fns {
		fn(args...)
	}
}
