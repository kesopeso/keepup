package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"keepup/apps/api/internal/auth"
)

func loadGoogle(c *AuthConfig) error {
	c.Google = auth.DefaultGoogleConfig()
	for name, dst := range map[string]*[]string{"AUTH_GOOGLE_WEB_CLIENT_IDS": &c.Google.WebClientIDs, "AUTH_GOOGLE_NATIVE_CLIENT_IDS": &c.Google.NativeClientIDs} {
		raw := os.Getenv(name)
		if strings.TrimSpace(raw) == "" {
			continue
		}
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				return fmt.Errorf("%s contains an empty client ID", name)
			}
			if !slices.Contains(*dst, id) {
				*dst = append(*dst, id)
			}
		}
	}
	var err error
	if c.Google.FlowTTL, err = durationOrDefault("AUTH_GOOGLE_FLOW_TTL", c.Google.FlowTTL); err != nil {
		return err
	}
	if c.Google.RequestTimeout, err = durationOrDefault("AUTH_GOOGLE_REQUEST_TIMEOUT", c.Google.RequestTimeout); err != nil {
		return err
	}
	return c.Google.Validate()
}
