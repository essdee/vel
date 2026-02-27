package hooks

import (
	"sync"
	"testing"
)

func TestAddFilterAndFilter(t *testing.T) {
	e := New()
	e.AddFilter("greet", func(data interface{}, ctx interface{}) interface{} {
		return data.(string) + " world"
	})
	result := e.Filter("greet", "hello", nil)
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %v", result)
	}
}

func TestMultipleFiltersChain(t *testing.T) {
	e := New()
	e.AddFilter("chain", func(data interface{}, ctx interface{}) interface{} {
		return data.(int) + 1
	})
	e.AddFilter("chain", func(data interface{}, ctx interface{}) interface{} {
		return data.(int) * 10
	})
	result := e.Filter("chain", 1, nil)
	if result != 20 {
		t.Fatalf("expected 20, got %v", result)
	}
}

func TestFilterWithNilData(t *testing.T) {
	e := New()
	e.AddFilter("nil", func(data interface{}, ctx interface{}) interface{} {
		if data == nil {
			return "was nil"
		}
		return data
	})
	result := e.Filter("nil", nil, nil)
	if result != "was nil" {
		t.Fatalf("expected 'was nil', got %v", result)
	}
}

func TestFilterOriginalUnchanged(t *testing.T) {
	e := New()
	e.AddFilter("copy", func(data interface{}, ctx interface{}) interface{} {
		return data.(string) + " modified"
	})
	original := "original"
	result := e.Filter("copy", original, nil)
	if original != "original" {
		t.Fatal("original was mutated")
	}
	if result != "original modified" {
		t.Fatalf("expected 'original modified', got %v", result)
	}
}

func TestFilterNoListeners(t *testing.T) {
	e := New()
	result := e.Filter("nope", "data", nil)
	if result != "data" {
		t.Fatalf("expected 'data', got %v", result)
	}
}

func TestOnAndEmit(t *testing.T) {
	e := New()
	var got []interface{}
	e.On("event", func(args ...interface{}) {
		got = args
	})
	e.Emit("event", "a", 42)
	if len(got) != 2 || got[0] != "a" || got[1] != 42 {
		t.Fatalf("expected [a 42], got %v", got)
	}
}

func TestMultipleActionsOnSameEvent(t *testing.T) {
	e := New()
	count := 0
	e.On("inc", func(args ...interface{}) { count++ })
	e.On("inc", func(args ...interface{}) { count++ })
	e.Emit("inc")
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestEmitNoListeners(t *testing.T) {
	e := New()
	e.Emit("nothing") // should not panic
}

func TestConcurrentAddFilterAndFilter(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			e.AddFilter("concurrent", func(data interface{}, ctx interface{}) interface{} {
				return data.(int) + 1
			})
		}()
		go func() {
			defer wg.Done()
			e.Filter("concurrent", 0, nil)
		}()
	}
	wg.Wait()
}
