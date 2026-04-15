package util

import (
	"os"

	"github.com/spf13/viper"
)

// GetBackendURL getafterendpointURL，priorityfromenvironmentvariableget，ifenvironmentvariableno存atthenfromconfigget
func GetBackendURL() string {
	// priorityfromenvironmentvariableget
	if backendURL := os.Getenv("BACKEND_URL"); backendURL != "" {
		return backendURL
	}
	// fromconfigfileget
	return viper.GetString("manager.backend_url")
}

