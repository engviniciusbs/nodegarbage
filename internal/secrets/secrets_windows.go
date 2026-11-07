//go:build windows

// Package secrets stores AI-provider API keys in Windows Credential Manager.
// Keys never touch files, SQLite or logs.
package secrets

import (
	"errors"
	"fmt"

	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

// ErrNotFound is returned by Get when no credential exists for the profile.
var ErrNotFound = errors.New("secrets: not found")

var errEmptyProfile = errors.New("secrets: empty profile id")

const (
	targetPrefix = "NodeGarbage/ai/"
	userName     = "NodeGarbage"
)

func target(profileID string) (string, error) {
	if profileID == "" {
		return "", errEmptyProfile
	}
	return targetPrefix + profileID, nil
}

// Set stores apiKey for the profile, replacing any existing value.
func Set(profileID, apiKey string) error {
	t, err := target(profileID)
	if err != nil {
		return err
	}
	c := wincred.NewGenericCredential(t)
	c.UserName = userName
	c.Persist = wincred.PersistLocalMachine
	c.CredentialBlob = []byte(apiKey)
	if err := c.Write(); err != nil {
		return fmt.Errorf("secrets: write credential: %w", err)
	}
	return nil
}

// Get returns the API key for the profile, or ErrNotFound.
func Get(profileID string) (string, error) {
	t, err := target(profileID)
	if err != nil {
		return "", err
	}
	c, err := wincred.GetGenericCredential(t)
	if err != nil {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("secrets: read credential: %w", err)
	}
	return string(c.CredentialBlob), nil
}

// Delete removes the profile's credential. A missing credential is not an error.
func Delete(profileID string) error {
	t, err := target(profileID)
	if err != nil {
		return err
	}
	c, err := wincred.GetGenericCredential(t)
	if err != nil {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("secrets: read credential: %w", err)
	}
	if err := c.Delete(); err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
		return fmt.Errorf("secrets: delete credential: %w", err)
	}
	return nil
}
