package execenv

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHermesTaskModelPreselectionPreservesProviderAndSource(t *testing.T) {
	source, home := t.TempDir(), t.TempDir()
	original := "model:\n  provider: vibe\n  default: previous-model\nproviders:\n  vibe:\n    base_url: https://example.invalid/v1\n"
	mustWrite(t, filepath.Join(source, "config.yaml"), original)
	readModel := func() (string, string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(home, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Model struct {
				Provider string `yaml:"provider"`
				Default  string `yaml:"default"`
			} `yaml:"model"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		return cfg.Model.Provider, cfg.Model.Default
	}
	for _, requested := range []string{"custom:vibe:first-model", "custom:vibe:second-model"} {
		if _, err := prepareHermesHome(home, source, false, nil, nil, "", "", testLogger()); err != nil {
			t.Fatal(err)
		}
		if err := pinHermesTaskModel(home, requested); err != nil {
			t.Fatal(err)
		}
		provider, model := readModel()
		if provider != "vibe" || "custom:vibe:"+model != requested {
			t.Fatalf("selected %s/%s for %s", provider, model, requested)
		}
		data, err := os.ReadFile(filepath.Join(source, "config.yaml"))
		if err != nil || string(data) != original {
			t.Fatalf("shared configuration changed: %v", err)
		}
	}
	if err := pinHermesTaskModel(home, "custom:another:third-model"); err != nil {
		t.Fatal(err)
	}
	provider, model := readModel()
	if provider != "vibe" || model != "second-model" {
		t.Fatalf("cross-provider request changed source selection: %s/%s", provider, model)
	}
}
