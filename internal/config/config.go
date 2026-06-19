// Package config resolves orbit's on-disk locations (home directory,
// config file, database) and loads the user's configuration.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// HomeEnv is the environment variable that, if set, overrides the
// default orbit home directory.
const HomeEnv = "ORBIT_HOME"

// ConfigFileName is the name of the config file inside the orbit home directory.
const ConfigFileName = "config.yaml"

// DatabaseFileName is the name of the SQLite database file inside the orbit home directory.
const DatabaseFileName = "orbit.db"

//go:embed default_config.yaml
var defaultConfig []byte

// Default returns the bytes of the commented default config file that
// `orbit init` writes into the orbit home directory. Every value is
// commented out, so a fresh install behaves the same whether or not
// the user has touched the file.
func Default() []byte {
	// Return a copy so callers cannot mutate the embedded slice.
	out := make([]byte, len(defaultConfig))
	copy(out, defaultConfig)
	return out
}

// Home returns the orbit home directory.
//
// If the ORBIT_HOME environment variable is set (and non-empty), its
// value is returned as an absolute path. Otherwise the default is
// <user-home>/.orbit.
//
// Home does not create the directory; that is the responsibility of
// `orbit init`.
func Home() (string, error) {
	if v := os.Getenv(HomeEnv); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("resolve %s=%q: %w", HomeEnv, v, err)
		}
		return abs, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user home directory: %w", err)
	}
	return filepath.Join(userHome, ".orbit"), nil
}

// ConfigPath returns the absolute path to the orbit config file
// (<home>/config.yaml). It does not check whether the file exists.
func ConfigPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigFileName), nil
}

// DatabasePath returns the absolute path to the orbit SQLite database
// (<home>/orbit.db). It does not check whether the file exists.
func DatabasePath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DatabaseFileName), nil
}
