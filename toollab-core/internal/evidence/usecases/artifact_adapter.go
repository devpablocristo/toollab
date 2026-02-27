package usecases

import (
	artifactUC "toollab-core/internal/artifact/usecases"
	"toollab-core/internal/shared"
)

type artifactPutterAdapter struct {
	svc *artifactUC.Service
}

func NewArtifactPutter(svc *artifactUC.Service) ArtifactPutter {
	return &artifactPutterAdapter{svc: svc}
}

func (a *artifactPutterAdapter) Put(runID string, artType shared.ArtifactType, rawJSON []byte) (int, error) {
	res, err := a.svc.Put(runID, artType, rawJSON)
	if err != nil {
		return 0, err
	}
	return res.Revision, nil
}
