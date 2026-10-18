//go:build vim4

package config

import "log"

func normalizePlatformConfig(c *Config) {
	c.Hardware = Hardware{Version: HWVersion(-1)}

	// Viper's weak string decoder maps YAML booleans to "0" and "1".
	// Normalize in memory without changing the user's authentication preference.
	switch c.Authentication {
	case "false", "0":
		c.Authentication = "disable"
	case "true", "1":
		c.Authentication = "enable"
	}

	// The manager owns public access and authentication, including for upgrades
	// that retain an older server.yaml. HTTP also disables TLS and its redirects.
	if c.Host != "127.0.0.1" || c.Port.Http != 8081 || c.Proto != "http" {
		log.Println("INFO: Using http://127.0.0.1:8081 because the vim4-manager gateway provides unified login")
	}
	c.Host = "127.0.0.1"
	c.Port.Http = 8081
	c.Proto = "http"
}
