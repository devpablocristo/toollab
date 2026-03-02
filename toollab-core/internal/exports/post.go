package exports

import (
	"encoding/json"
	"log"

	artifactUC "toollab-core/internal/artifact"
	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/intelligence"
	"toollab-core/internal/shared"
)

// GenerateExportsPost creates endpoint intelligence + all exports. Called by orchestrator.
func GenerateExportsPost(artifactSvc *artifactUC.Service, runID string, dossier *d.DossierV2Full) d.ExportsIndex {
	idx := dossier.ExportsIndex

	intel := intelligence.Generate(dossier)

	saveJSON := func(artType shared.ArtifactType, data []byte) {
		if _, err := artifactSvc.Put(runID, artType, data); err != nil {
			log.Printf("save export %s: %v", artType, err)
		}
	}
	saveRaw := func(artType shared.ArtifactType, data []byte) {
		if _, err := artifactSvc.PutRaw(runID, artType, data); err != nil {
			log.Printf("save export %s: %v", artType, err)
		}
	}

	if intelJSON, err := json.Marshal(intel); err == nil {
		saveJSON(shared.ArtifactEndpointIntelligence, intelJSON)
	}

	intelIdx := intelligence.GenerateIndex(intel)
	if idxJSON, err := json.Marshal(intelIdx); err == nil {
		saveJSON(shared.ArtifactEndpointIntelIndex, idxJSON)
	}

	queryScripts := intelligence.GenerateQueryScripts(intel)
	if qsJSON, err := json.Marshal(queryScripts); err == nil {
		saveJSON(shared.ArtifactEndpointQueries, qsJSON)
	}

	postmanData := GeneratePostmanCollection(dossier, intel)
	saveJSON(shared.ArtifactPostmanCollection, postmanData)
	idx.PostmanCollection = "postman_collection.json"

	curlData := GenerateCurlBook(dossier, intel)
	saveJSON(shared.ArtifactCurlBook, curlData)
	idx.CurlBook = "curl_book.json"

	openapiData := GenerateOpenAPIInferred(dossier, intel)
	saveRaw(shared.ArtifactOpenAPIInferred, openapiData)
	idx.OpenAPIInferred = "openapi_inferred.yaml"

	openapiAST := GenerateOpenAPIAST(dossier)
	saveRaw(shared.ArtifactOpenAPIAST, openapiAST)

	if len(dossier.Scoring.Hotspots) > 0 {
		data := GenerateHotspotsCSV(dossier.Scoring.Hotspots)
		saveRaw(shared.ArtifactScoring, data)
		idx.HotspotsCSV = "endpoint_hotspots.csv"
	}

	envExample := GenerateEnvExample(dossier)
	saveRaw(shared.ArtifactEnvExample, envExample)
	idx.EnvExample = ".env.example"

	runSummaryJSON, _ := json.Marshal(dossier.RunSummary)
	saveJSON(shared.ArtifactRunSummaryExport, runSummaryJSON)

	return idx
}
