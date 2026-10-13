//go:build vim4

package config

// Viper's weak string decoder maps YAML booleans to "0" and "1".
// Normalize in memory without changing the user's authentication preference.
func normalizePlatformConfig(c *Config) {
	c.Hardware = Hardware{Version: HWVersion(-1)}
	switch c.Authentication {
	case "false", "0":
		c.Authentication = "disable"
	case "true", "1":
		c.Authentication = "enable"
	}
}
