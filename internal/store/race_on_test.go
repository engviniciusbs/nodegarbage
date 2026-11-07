//go:build race

package store

// raceEnabled reports whether the test binary was built with -race,
// which slows SQLite inserts by roughly 10x.
const raceEnabled = true
