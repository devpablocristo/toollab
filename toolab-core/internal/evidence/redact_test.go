package evidence

import (
	"strings"
	"testing"
)

func TestRedactionNoLeaks(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer secret",
		"x-api-key":     "secret-key",
		"x-safe":        "value",
	}
	redactedHeaders := RedactHeaders(headers, []string{"authorization", "x-api-key"}, "***")
	if redactedHeaders["Authorization"] != "***" || redactedHeaders["x-api-key"] != "***" {
		t.Fatalf("sensitive headers not redacted")
	}
	if redactedHeaders["x-safe"] != "value" {
		t.Fatalf("safe header should remain")
	}

	body := []byte(`{"user":"alice","password":"secret","nested":{"token":"abc"}}`)
	preview := RedactBodyPreview(body, []string{"$.password", "$.nested.token"}, "***", 4096)
	if strings.Contains(preview, "secret") || strings.Contains(preview, "abc") {
		t.Fatalf("body preview leaked sensitive content: %s", preview)
	}
}
