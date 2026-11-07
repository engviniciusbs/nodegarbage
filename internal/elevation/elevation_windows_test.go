//go:build windows

package elevation

import "testing"

func TestIsElevatedDoesNotPanic(t *testing.T) {
	t.Logf("IsElevated = %v", IsElevated())
}
