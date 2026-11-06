//go:build windows

package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
)

func randomProfile(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "test-" + hex.EncodeToString(b)
}

func TestRoundTrip(t *testing.T) {
	id := randomProfile(t)
	t.Cleanup(func() { _ = Delete(id) })

	const key = "sk-test-ключ-123"
	if err := Set(id, key); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != key {
		t.Fatalf("Get = %q, want %q", got, key)
	}
	if err := Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteMissingIsNil(t *testing.T) {
	if err := Delete(randomProfile(t)); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestEmptyProfileID(t *testing.T) {
	if err := Set("", "k"); err == nil {
		t.Error("Set with empty id: want error")
	}
	if _, err := Get(""); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("Get with empty id: err = %v, want non-NotFound error", err)
	}
	if err := Delete(""); err == nil {
		t.Error("Delete with empty id: want error")
	}
}
