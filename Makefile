.PHONY: up down be fe test build clean

# Levanta todo (backend + frontend) en paralelo
up:
	@docker compose up --build -d
	@echo "Backend: http://localhost:8090"
	@echo "Frontend: http://localhost:5173"
	@echo "Run 'make down' to stop."

down:
	@docker compose down

# Dev sin docker (manual)
dev:
	@echo "Starting backend + frontend in dev mode..."
	@$(MAKE) -j2 be fe

be:
	@cd toollab-core && mkdir -p data && CGO_ENABLED=1 go run ./cmd/toollab-dashboard

fe:
	@cd toollab-ui && npm run dev

# Tests
test:
	@cd toollab-core && CGO_ENABLED=1 go test ./... -v -count=1

test-quick:
	@cd toollab-core && CGO_ENABLED=1 go test ./... -count=1

# Build
build:
	@cd toollab-core && CGO_ENABLED=1 go build -o ../bin/toollab-dashboard ./cmd/toollab-dashboard
	@cd toollab-ui && npm run build

# Install deps
install:
	@cd toollab-ui && npm install

clean:
	rm -rf bin/ toollab-core/data/ toollab-ui/dist/
