//go:build vim4

package config

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestManagerGatewayOverridesLegacyWebConfig(t *testing.T) {
	var messages bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&messages)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(`host: 0.0.0.0
port: {http: 8080, https: 8443}
proto: https
jwt: {secretKey: private-key}
`)); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	normalizePlatformConfig(&c)
	if c.Host != "127.0.0.1" || c.Port.Http != 8081 || c.Proto != "http" {
		t.Fatalf("legacy web settings escaped the manager gateway: %+v", c)
	}
	// The stored TLS port remains a user setting, but Proto prevents both the
	// TLS listener and HTTP-to-HTTPS redirect path in run() from starting.
	if c.Port.Https != 8443 || c.JWT.SecretKey != "private-key" {
		t.Fatal("gateway overrides changed unrelated configuration")
	}
	if strings.Count(messages.String(), "\n") != 1 {
		t.Fatalf("expected one gateway override notice, got %q", messages.String())
	}
	messages.Reset()
	normalizePlatformConfig(&c)
	if messages.Len() != 0 {
		t.Fatal("unchanged gateway settings should not log an override")
	}
}

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
