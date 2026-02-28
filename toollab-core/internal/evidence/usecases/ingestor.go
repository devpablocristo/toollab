package usecases

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	artifactDomain "toollab-core/internal/artifact/usecases/domain"
	"toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/shared"
)

const maxInlineBody = 4096

type FSIngestor struct {
	storage artifactDomain.ContentStorage
	artPut  ArtifactPutter
}

type ArtifactPutter interface {
	Put(runID string, artType shared.ArtifactType, rawJSON []byte) (revision int, err error)
}

func NewFSIngestor(storage artifactDomain.ContentStorage, putter ArtifactPutter) *FSIngestor {
	return &FSIngestor{storage: storage, artPut: putter}
}

func (ing *FSIngestor) Ingest(runID string, exec domain.ExecutionResult) (domain.EvidencePack, int, error) {
	pack := domain.EvidencePack{
		PackID:    shared.NewID(),
		RunID:     runID,
		CreatedAt: shared.Now(),
		Items:     make([]domain.EvidenceItem, 0, len(exec.Cases)),
	}

	for _, cr := range exec.Cases {
		item, err := ing.buildItem(runID, cr)
		if err != nil {
			return domain.EvidencePack{}, 0, fmt.Errorf("building evidence item for case %s: %w", cr.CaseID, err)
		}
		pack.Items = append(pack.Items, item)
	}

	packJSON, err := json.Marshal(pack)
	if err != nil {
		return domain.EvidencePack{}, 0, fmt.Errorf("marshaling evidence pack: %w", err)
	}

	rev, err := ing.artPut.Put(runID, shared.ArtifactEvidencePack, packJSON)
	if err != nil {
		return domain.EvidencePack{}, 0, fmt.Errorf("saving evidence pack artifact: %w", err)
	}

	return pack, rev, nil
}

func (ing *FSIngestor) buildItem(runID string, cr domain.CaseResult) (domain.EvidenceItem, error) {
	item := domain.EvidenceItem{
		EvidenceID: cr.EvidenceID,
		CaseID:     cr.CaseID,
		Kind:       "http_exchange",
		Tags:       cr.Tags,
		TimingMs:   cr.TimingMs,
		Error:      cr.Error,
		Request: domain.EvidenceRequest{
			Method:  cr.ReqFinal.Method,
			URL:     cr.ReqFinal.URL,
			Headers: MaskHeaders(cr.ReqFinal.Headers),
		},
	}

	var hashes domain.EvidenceHashes

	if len(cr.ReqFinal.BodyRaw) > 0 {
		ref := rawPath(runID, cr.EvidenceID, "request.body")
		if err := ing.storage.Write(ref, cr.ReqFinal.BodyRaw); err != nil {
			return domain.EvidenceItem{}, fmt.Errorf("writing request body: %w", err)
		}
		item.Request.BodyRef = ref
		hashes.SHA256RequestBody = shared.SHA256Bytes(cr.ReqFinal.BodyRaw)

		if len(cr.ReqFinal.BodyRaw) <= maxInlineBody {
			item.Request.BodyInlineTruncated = string(cr.ReqFinal.BodyRaw)
		} else {
			item.Request.BodyInlineTruncated = string(cr.ReqFinal.BodyRaw[:maxInlineBody]) + "...[truncated]"
		}
	}

	if cr.Response != nil {
		item.Response = &domain.EvidenceResponse{
			Status:  cr.Response.Status,
			Headers: MaskHeaders(cr.Response.Headers),
		}

		if len(cr.Response.BodyRaw) > 0 {
			ref := rawPath(runID, cr.EvidenceID, "response.body")
			if err := ing.storage.Write(ref, cr.Response.BodyRaw); err != nil {
				return domain.EvidenceItem{}, fmt.Errorf("writing response body: %w", err)
			}
			item.Response.BodyRef = ref
			hashes.SHA256ResponseBody = shared.SHA256Bytes(cr.Response.BodyRaw)

			if len(cr.Response.BodyRaw) <= maxInlineBody {
				item.Response.BodyInlineTruncated = string(cr.Response.BodyRaw)
			} else {
				item.Response.BodyInlineTruncated = string(cr.Response.BodyRaw[:maxInlineBody]) + "...[truncated]"
			}
		}
	}

	if hashes.SHA256RequestBody != "" || hashes.SHA256ResponseBody != "" {
		item.Hashes = &hashes
	}

	return item, nil
}

func rawPath(runID, evidenceID, suffix string) string {
	return filepath.Join("runs", runID, "evidence", "raw", evidenceID+"."+suffix)
}
