package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Version             string
	Listen              string
	DataDir             string
	WebRoot             string
	SecureCookies       bool
	EnableDockerActions bool
	EnablePowerActions  bool
	PowerHelperSocket   string
	SessionTTL          time.Duration
}

func FromEnv() Config {
	return Config{
		Version:             "dev",
		Listen:              env("SERVERPANEL_LISTEN", "127.0.0.1:9080"),
		DataDir:             env("SERVERPANEL_DATA_DIR", filepath.Join(".", "data")),
		WebRoot:             env("SERVERPANEL_WEB_ROOT", filepath.Join(".", "web")),
		SecureCookies:       envBool("SERVERPANEL_SECURE_COOKIES", true),
		EnableDockerActions: envBool("SERVERPANEL_ENABLE_DOCKER_ACTIONS", false),
		EnablePowerActions:  envBool("SERVERPANEL_ENABLE_POWER_ACTIONS", false),
		PowerHelperSocket:   env("SERVERPANEL_POWER_HELPER_SOCKET", "/run/serverpanel-power/power.sock"),
		SessionTTL:          12 * time.Hour,
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
