package util

import (
	"os"

	"github.com/spf13/viper"
)

// GetBackendURL get backend endpoint URL, priority from environment variable, if environment variable not exist then from config get
func GetBackendURL() string {
	// priority from environment variable
	if backendURL := os.Getenv("BACKEND_URL"); backendURL != "" {
		return backendURL
	}
	// from config file get
	return viper.GetString("manager.backend_url")
}
