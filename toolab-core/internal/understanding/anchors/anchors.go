package anchors

import (
	"fmt"
	"regexp"
)

var jsonPointerPattern = regexp.MustCompile(`^/`)

type Anchor struct {
	Type  string
	Value string
}

func Validate(anchor Anchor) error {
	if anchor.Type == "" || anchor.Value == "" {
		return fmt.Errorf("invalid anchor: missing type/value")
	}
	switch anchor.Type {
	case "seq", "request_id", "request_fingerprint", "response_hash":
		return nil
	case "json_pointer":
		if !jsonPointerPattern.MatchString(anchor.Value) {
			return fmt.Errorf("invalid json_pointer anchor")
		}
		return nil
	default:
		return fmt.Errorf("unsupported anchor type %q", anchor.Type)
	}
}
