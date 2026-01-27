package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Model struct {
	Name  string `yaml:"name"`
	Model string `yaml:"model"`
}

type UI struct {
	HideGlobalProjects       bool `yaml:"hide_global_projects"`
	GlobalSessionsMaxAgeDays int  `yaml:"global_sessions_max_age_days"`
}

type Config struct {
	DefaultModel string  `yaml:"default_model"`
	Models       []Model `yaml:"models"`
	UI           UI      `yaml:"ui"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}
	// Ensure there are no extra YAML documents.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid yaml: multiple documents are not supported")
		}
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}

	if len(cfg.Models) == 0 {
		return nil, errors.New("no models configured")
	}
	if cfg.UI.GlobalSessionsMaxAgeDays < 0 {
		return nil, fmt.Errorf("ui.global_sessions_max_age_days must be >= 0")
	}
	for i, m := range cfg.Models {
		if strings.TrimSpace(m.Name) == "" {
			return nil, fmt.Errorf("models[%d].name is required", i)
		}
		if strings.TrimSpace(m.Model) == "" {
			return nil, fmt.Errorf("models[%d].model is required", i)
		}
	}

	return &cfg, nil
}

func (c *Config) Default() (Model, error) {
	if c == nil || len(c.Models) == 0 {
		return Model{}, errors.New("no models available")
	}
	if strings.TrimSpace(c.DefaultModel) == "" {
		return c.Models[0], nil
	}

	needle := strings.ToLower(strings.TrimSpace(c.DefaultModel))
	for _, m := range c.Models {
		if strings.ToLower(m.Name) == needle {
			return m, nil
		}
	}
	for _, m := range c.Models {
		if strings.ToLower(m.Model) == needle {
			return m, nil
		}
	}
	return Model{}, fmt.Errorf("default_model %q does not match any configured model", c.DefaultModel)
}

func MinimalExampleYAML() string {
	// Keep this tiny; users can add more models.
	return `
default_model: Gemini Pro
ui:
  hide_global_projects: false
  global_sessions_max_age_days: 0
models:
  - name: Gemini Pro
    model: google/gemini-3-pro-preview
  - name: GPT-5.2
    model: openai/gpt-5.2
  - name: Gemini Flash
    model: google/gemini-flash
  - name: GPT-5.1
    model: openai/gpt-5.1
`
}
