//go:build vim4

package config

import (
	"github.com/spf13/viper"
	"strings"
	"testing"
)

func TestLegacyAuthenticationPreservesDisabledLogin(t *testing.T) {
	for _, value := range []string{"false", "disable", "true", "enable"} {
		v := viper.New()
		v.SetConfigType("yaml")
		if err := v.ReadConfig(strings.NewReader("authentication: " + value + "\n")); err != nil {
			t.Fatal(err)
		}
		var c Config
		if err := v.Unmarshal(&c); err != nil {
			t.Fatal(err)
		}
		normalizePlatformConfig(&c)
		disabled := value == "false" || value == "disable"
		if (c.Authentication == "disable") != disabled {
			t.Fatalf("%s became %s", value, c.Authentication)
		}
		if c.Hardware.GPIOPower != "" {
			t.Fatal("Ubuntu inherited appliance GPIO")
		}
	}
}
