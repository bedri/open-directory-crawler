package envutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGetFallback(t *testing.T) {
	v := Get("NONEXISTENT_VAR_12345", "default")
	if v != "default" {
		t.Errorf("got %q, want %q", v, "default")
	}
}

func TestGetFromEnv(t *testing.T) {
	os.Setenv("ODK_TEST_KEY", "from_env")
	defer os.Unsetenv("ODK_TEST_KEY")
	v := Get("ODK_TEST_KEY", "fallback")
	if v != "from_env" {
		t.Errorf("got %q, want %q", v, "from_env")
	}
}

func TestGetFromDotEnv(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	os.WriteFile(filepath.Join(dir, ".env"), []byte("ODK_DOTENV_KEY=from_dotenv\n"), 0644)
	loaded = sync.Once{} // reset for test

	v := Get("ODK_DOTENV_KEY", "fallback")
	if v != "from_dotenv" {
		t.Errorf("got %q, want %q", v, "from_dotenv")
	}
}

func TestGetDotEnvOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	os.Setenv("ODK_OVERRIDE_TEST", "from_env")
	defer os.Unsetenv("ODK_OVERRIDE_TEST")

	os.WriteFile(filepath.Join(dir, ".env"), []byte("ODK_OVERRIDE_TEST=from_dotenv\n"), 0644)
	loaded = sync.Once{} // reset for test

	v := Get("ODK_OVERRIDE_TEST", "fallback")
	if v != "from_dotenv" {
		t.Errorf("got %q, want %q", v, "from_dotenv")
	}
}

func TestGetIgnoresComments(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	os.WriteFile(filepath.Join(dir, ".env"), []byte("# comment\nKEY=value\n"), 0644)
	loaded = sync.Once{}

	v := Get("KEY", "")
	if v != "value" {
		t.Errorf("got %q, want %q", v, "value")
	}
}

func TestGetEmptyLines(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	os.WriteFile(filepath.Join(dir, ".env"), []byte("\n\nKEY=val\n\n"), 0644)
	loaded = sync.Once{}

	v := Get("KEY", "")
	if v != "val" {
		t.Errorf("got %q, want %q", v, "val")
	}
}
