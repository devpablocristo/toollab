SHELL := /bin/bash

.PHONY: test fmt run

test:
	cd toolab-core && go test ./...

fmt:
	cd toolab-core && gofmt -w .

run:
	cd toolab-core && go run ./cmd/toolab run ../testdata/e2e/scenario.yaml
