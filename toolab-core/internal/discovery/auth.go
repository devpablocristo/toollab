package discovery

import (
	"fmt"
	"os"
	"strings"
)

func ParseAuthFlag(raw string) (*AuthConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid auth format")
	}

	switch parts[0] {
	case "bearer":
		if len(parts) != 2 {
			return nil, fmt.Errorf("bearer auth format: bearer:ENV_VAR")
		}
		return &AuthConfig{Kind: "bearer", EnvVar: parts[1]}, nil
	case "api_key":
		if len(parts) != 4 {
			return nil, fmt.Errorf("api_key auth format: api_key:ENV_VAR:header|query:NAME")
		}
		loc := parts[2]
		if loc != "header" && loc != "query" {
			return nil, fmt.Errorf("api_key location must be header or query")
		}
		return &AuthConfig{
			Kind:     "api_key",
			EnvVar:   parts[1],
			Location: loc,
			Name:     parts[3],
		}, nil
	default:
		return nil, fmt.Errorf("unsupported auth kind %q", parts[0])
	}
}

func (a *AuthConfig) EnvValue() string {
	if a == nil || a.EnvVar == "" {
		return ""
	}
	return os.Getenv(a.EnvVar)
}
