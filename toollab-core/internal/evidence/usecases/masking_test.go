package usecases

import "testing"

func TestMaskHeaders(t *testing.T) {
	input := map[string]string{
		"Authorization": "Bearer secret",
		"Cookie":        "session=abc",
		"X-Api-Key":     "key123",
		"Content-Type":  "application/json",
		"Accept":        "*/*",
	}

	masked := MaskHeaders(input)

	if masked["Authorization"] != "***MASKED***" {
		t.Fatal("Authorization should be masked")
	}
	if masked["Cookie"] != "***MASKED***" {
		t.Fatal("Cookie should be masked")
	}
	if masked["X-Api-Key"] != "***MASKED***" {
		t.Fatal("X-Api-Key should be masked")
	}
	if masked["Content-Type"] != "application/json" {
		t.Fatal("Content-Type should NOT be masked")
	}
	if masked["Accept"] != "*/*" {
		t.Fatal("Accept should NOT be masked")
	}
}

func TestMaskHeaders_Nil(t *testing.T) {
	if MaskHeaders(nil) != nil {
		t.Fatal("nil input should return nil")
	}
}
