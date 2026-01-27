package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "oc-config.yaml")

	if err := os.WriteFile(p, []byte(`
default_model: GPT-5.2
ui:
  hide_global_projects: false
  global_sessions_max_age_days: 0
models:
  - name: Gemini Pro
    model: google/gemini-pro
  - name: GPT-5.2
    model: openai/gpt-5.2
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	def, err := cfg.Default()
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "GPT-5.2" {
		t.Fatalf("expected default GPT-5.2, got %q", def.Name)
	}
}

func TestLoad_RejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "oc-config.yaml")

	// Misspelled key: global_session_max_age_days (missing 's')
	if err := os.WriteFile(p, []byte(`
default_model: GPT-5.2
ui:
  global_session_max_age_days: 5
models:
  - name: GPT-5.2
    model: openai/gpt-5.2
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(p)
	if err == nil {
		t.Fatalf("expected error for unknown fields")
	}
}
