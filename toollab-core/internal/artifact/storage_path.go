package artifact

import (
	"fmt"
	"path/filepath"
)

func StoragePath(runID, artifactType string, revision int) string {
	return filepath.Join("runs", runID, artifactType, fmt.Sprintf("v%d.json", revision))
}
