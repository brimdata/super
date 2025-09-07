package dbflags

import (
	"os"
	"path/filepath"
	"runtime"
)

// getDefaultDataDir returns the default data directory for the current user.
// Derived from https://github.com/btcsuite/btcd/blob/master/btcutil/appdata.go
func getDefaultDataDir() string {
	// Resolve the XDG data home directory if set.
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "super")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			return filepath.Join(appData, "super")
		}
	}
	if homeDir, _ := os.UserHomeDir(); homeDir != "" {
		// Follow the XDG spec which states:
		// If $XDG_DATA_HOME is either not set or empty, a default equal to
		// $HOME/.local/share should be used.
		return filepath.Join(homeDir, ".local", "share", "super")
	}
	// Return an empty string which will cause an error if a default data
	// directory cannot be found.
	return ""
}
