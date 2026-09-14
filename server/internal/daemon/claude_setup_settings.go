package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

const claudeRepositorySetupSettingsFile = "claude-repository-setup-settings.json"
const claudeSetupSettingsMaxBytes = 1 << 20

func prepareClaudeSetupSettings(envRoot, existingSettingsPath string, roots []string) (string, error) {
	if len(roots) == 0 {
		return existingSettingsPath, nil
	}
	if !filepath.IsAbs(envRoot) {
		return "", fmt.Errorf("Claude setup cache grants require a task environment")
	}
	root, err := filepath.EvalSymlinks(envRoot)
	if err != nil {
		return "", err
	}

	settingsPath := existingSettingsPath
	settings := make(map[string]any)
	if settingsPath == "" {
		settingsPath = filepath.Join(root, claudeRepositorySetupSettingsFile)
	} else {
		if !filepath.IsAbs(settingsPath) {
			return "", fmt.Errorf("Claude setup cache settings must be task-local")
		}
		cleaned := filepath.Clean(settingsPath)
		if filepath.Dir(cleaned) != filepath.Clean(envRoot) {
			return "", fmt.Errorf("Claude setup cache settings escaped the task environment")
		}
		info, statErr := os.Lstat(cleaned)
		if statErr != nil {
			return "", statErr
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("Claude setup cache settings must be a regular file")
		}
		// Windows Mode reports DOS read-only bits, not POSIX owner permissions.
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			return "", fmt.Errorf("Claude setup cache settings must have mode 0600")
		}
		if info.Size() > claudeSetupSettingsMaxBytes {
			return "", fmt.Errorf("Claude setup cache settings exceed the size limit")
		}
		canonical, evalErr := filepath.EvalSymlinks(cleaned)
		if evalErr != nil {
			return "", fmt.Errorf("resolve Claude setup cache settings: %w", evalErr)
		}
		if canonical != filepath.Join(root, filepath.Base(cleaned)) {
			return "", fmt.Errorf("Claude setup cache settings resolved outside the task environment")
		}
		data, readErr := os.ReadFile(cleaned)
		if readErr != nil {
			return "", readErr
		}
		if decodeErr := json.Unmarshal(data, &settings); decodeErr != nil {
			return "", fmt.Errorf("decode Claude setup cache settings: %w", decodeErr)
		}
		if settings == nil {
			return "", fmt.Errorf("decode Claude setup cache settings: expected an object")
		}
	}
	sandbox, err := claudeSettingsObject(settings, "sandbox")
	if err != nil {
		return "", err
	}
	filesystem, err := claudeSettingsObject(sandbox, "filesystem")
	if err != nil {
		return "", err
	}
	allowWrite, err := claudeSettingsStringList(filesystem, "allowWrite")
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		if !slices.Contains(allowWrite, root) {
			allowWrite = append(allowWrite, root)
		}
	}
	filesystem["allowWrite"] = allowWrite

	merged, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(settingsPath), ".claude-setup-settings-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(merged); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, settingsPath); err != nil {
		return "", err
	}
	return settingsPath, nil
}

func claudeSettingsObject(parent map[string]any, key string) (map[string]any, error) {
	value, exists := parent[key]
	if !exists {
		object := make(map[string]any)
		parent[key] = object
		return object, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Claude setup cache settings %s must be an object", key)
	}
	return object, nil
}

func claudeSettingsStringList(parent map[string]any, key string) ([]string, error) {
	value, exists := parent[key]
	if !exists {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Claude setup cache settings %s must be an array", key)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("Claude setup cache settings %s must contain strings", key)
		}
		result = append(result, text)
	}
	return result, nil
}
