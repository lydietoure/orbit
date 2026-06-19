package config

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestHome_UsesEnvWhenSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)

	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if got != dir {
		t.Errorf("Home = %q, want %q", got, dir)
	}
}

func TestHome_EnvIsMadeAbsolute(t *testing.T) {
	t.Setenv(HomeEnv, "relative/path")

	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("Home = %q, want absolute path", got)
	}
}

func TestHome_EmptyEnvFallsBackToUserHome(t *testing.T) {
	t.Setenv(HomeEnv, "")
	// Pin HOME / USERPROFILE so the fallback is deterministic.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome) // Windows

	got, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	want := filepath.Join(fakeHome, ".orbit")
	if got != want {
		t.Errorf("Home = %q, want %q", got, want)
	}
}

func TestDefault_NonEmpty(t *testing.T) {
	b := Default()
	if len(b) == 0 {
		t.Fatal("Default() returned empty bytes")
	}
}

func TestDefault_MentionsScratchpadRoot(t *testing.T) {
	b := Default()
	if !bytes.Contains(b, []byte("scratchpad:")) {
		t.Error("Default() missing 'scratchpad:' section")
	}
	if !bytes.Contains(b, []byte("root:")) {
		t.Error("Default() missing 'root:' key")
	}
}

func TestDefault_ReturnsCopy(t *testing.T) {
	a := Default()
	if len(a) == 0 {
		t.Fatal("Default() returned empty bytes")
	}
	a[0] = 'X'
	b := Default()
	if b[0] == 'X' {
		t.Error("Default() returned a slice that aliases the embedded bytes; mutation leaked")
	}
}

func TestConfigPath_JoinsHomeWithConfigFileName(t *testing.T) {
	// bit useless, no?
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)

	got, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	want := filepath.Join(dir, ConfigFileName)
	if got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}


func TestDatabasePath_JoinsHomeWithDatabaseFileName(t *testing.T) {
	// TODO: bit useless, no?
	dir := t.TempDir()
	t.Setenv(HomeEnv, dir)

	got, err := DatabasePath()
	if err != nil {
		t.Fatalf("DatabasePath: %v", err)
	}
	want := filepath.Join(dir, DatabaseFileName)
	if got != want {
		t.Errorf("DatabasePath = %q, want %q", got, want)
	}
}
