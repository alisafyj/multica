package agent

import "fmt"

func validateProjectConfigurationPolicy(policy string) error {
	switch policy {
	case "", "restricted", "trusted":
		return nil
	default:
		return fmt.Errorf("invalid project configuration policy")
	}
}
