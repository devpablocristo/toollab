SHELL := /bin/bash

.PHONY: test fmt run

test:
	cd toollab-core && go test ./...

fmt:
	cd toollab-core && gofmt -w .

run:
	cd toollab-core && go run ./cmd/toollab run ../testdata/e2e/scenario.yaml
