.PHONY: run test build clean

run:
	@mkdir -p data
	CGO_ENABLED=1 go run ./cmd/toollab-dashboard

build:
	CGO_ENABLED=1 go build -o bin/toollab-dashboard ./cmd/toollab-dashboard

test:
	CGO_ENABLED=1 go test ./... -v -count=1

clean:
	rm -rf bin/ data/
