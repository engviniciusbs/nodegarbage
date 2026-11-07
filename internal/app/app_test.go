package app

import (
	"reflect"
	"testing"
)

// Lifecycle hooks must not be exported methods: Wails would bind them to JS.
func TestNoLifecycleBindings(t *testing.T) {
	typ := reflect.TypeOf(&App{})
	for _, name := range []string{"Startup", "Shutdown", "DomReady"} {
		if _, ok := typ.MethodByName(name); ok {
			t.Errorf("App.%s is exported and would be bound to the frontend", name)
		}
	}
}
