package models

type ArtifactRow struct {
	ID            string
	RunID         string
	Type          string
	SchemaVersion string
	Revision      int
	ContentHash   string
	SizeBytes     int64
	StoragePath   string
	CreatedAt     string
}
