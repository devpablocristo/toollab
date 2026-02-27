package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"toollab-v2/internal/shared/common"
	"toollab-v2/internal/shared/model"
)

func BuildSnapshot(sourceType, sourceRef, localPath string) (model.RepoSnapshot, error) {
	paths := make([]model.RepoFile, 0, 1024)
	root := filepath.Clean(localPath)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		if info.Size() > 2*1024*1024 {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		sum, herr := hashFile(path)
		if herr != nil {
			return nil
		}
		paths = append(paths, model.RepoFile{
			Path:   filepath.ToSlash(rel),
			Size:   info.Size(),
			SHA256: sum,
		})
		return nil
	})
	if err != nil {
		return model.RepoSnapshot{}, err
	}

	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	treeBuilder := strings.Builder{}
	for _, f := range paths {
		treeBuilder.WriteString(f.Path)
		treeBuilder.WriteByte(':')
		treeBuilder.WriteString(f.SHA256)
		treeBuilder.WriteByte('\n')
	}

	snapshotID := common.SHA256String(sourceRef + ":" + treeBuilder.String())
	return model.RepoSnapshot{
		SnapshotID:        snapshotID,
		SourceType:        sourceType,
		RepoName:          repoNameFromRef(sourceRef, root),
		HashTree:          common.SHA256String(treeBuilder.String()),
		CreatedAt:         time.Now().UTC(),
		Files:             paths,
		ResolvedLocalPath: root,
	}, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func repoNameFromRef(sourceRef, localPath string) string {
	if sourceRef != "" {
		parts := strings.Split(strings.TrimSuffix(sourceRef, ".git"), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return filepath.Base(localPath)
}
