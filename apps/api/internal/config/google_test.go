package config

import (
	"testing"
	"time"
)

func TestGoogleConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, web, native, ttl, timeout string
		valid                           bool
	}{
		{name: "optional", valid: true},
		{name: "web only", web: "web.apps.googleusercontent.com", valid: true},
		{name: "native only", native: "native.apps.googleusercontent.com", valid: true},
		{name: "both", web: "web.apps.googleusercontent.com", native: "native.apps.googleusercontent.com,backend.apps.googleusercontent.com", ttl: "5m", timeout: "3s", valid: true},
		{name: "malformed", web: "https://example.com"},
		{name: "empty element", web: "web.apps.googleusercontent.com,"},
		{name: "bad ttl", ttl: "0s"},
		{name: "bad timeout", timeout: "-1s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUTH_GOOGLE_WEB_CLIENT_IDS", tc.web)
			t.Setenv("AUTH_GOOGLE_NATIVE_CLIENT_IDS", tc.native)
			t.Setenv("AUTH_GOOGLE_FLOW_TTL", tc.ttl)
			t.Setenv("AUTH_GOOGLE_REQUEST_TIMEOUT", tc.timeout)
			var cfg AuthConfig
			err := loadGoogle(&cfg)
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if tc.valid && tc.ttl == "" && cfg.Google.FlowTTL != 10*time.Minute {
				t.Fatal("wrong default")
			}
		})
	}
}
