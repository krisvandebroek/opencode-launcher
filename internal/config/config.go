package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Model struct {
	Name  string `yaml:"name"`
	Model string `yaml:"model"`
}

type Config struct {
	DefaultModel string  `yaml:"default_model"`
	Models       []Model `yaml:"models"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}

	if len(cfg.Models) == 0 {
		return nil, errors.New("no models configured")
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
