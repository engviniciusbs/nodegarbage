//go:build windows

// Package elevation detects and requests administrator rights.
package elevation

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process runs with an elevated token.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RestartElevated launches the current executable via the "runas" verb (UAC
// prompt) with args. The caller is responsible for exiting the app afterwards.
func RestartElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	argp, err := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, argp, dir, windows.SW_NORMAL)
}
