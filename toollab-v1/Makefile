SHELL := /bin/bash

CORE_DIR := toollab-core
DASHBOARD_DIR := toollab-dashboard
UI_DIR := toollab-ui

.PHONY: up build down clean logs \
	core-test core-fmt core-build core-dev \
	dashboard-test dashboard-build dashboard-dev \
	ui-dev ui-build ui-test \
	qa reset

# ─── Docker ──────────────────────────────────────────

up:
	docker compose up -d --remove-orphans

build:
	docker compose build

down:
	docker compose down --remove-orphans

clean:
	docker compose down -v --remove-orphans

logs:
	docker compose logs -f

logs-tail:
	docker compose logs --tail=$${TAIL:-200}

reset:
	$(MAKE) clean
	$(MAKE) build
	$(MAKE) up
	@echo "Toollab running:"
	@echo "  Dashboard API: http://localhost:8090"
	@echo "  UI:            http://localhost:5173"

# ─── Core ────────────────────────────────────────────

core-test:
	cd $(CORE_DIR) && go test ./...

core-fmt:
	cd $(CORE_DIR) && gofmt -w .

core-build:
	cd $(CORE_DIR) && go build -o toollab ./cmd/toollab

core-dev:
	cd $(CORE_DIR) && go run ./cmd/toollab $(CMD)

# ─── Dashboard ───────────────────────────────────────

dashboard-test:
	cd $(DASHBOARD_DIR) && go test ./...

dashboard-build:
	cd $(DASHBOARD_DIR) && go build -o toollab-dashboard ./cmd/api

dashboard-dev:
	cd $(DASHBOARD_DIR) && go run ./cmd/api

# ─── UI ──────────────────────────────────────────────

ui-dev:
	cd $(UI_DIR) && npm run dev

ui-build:
	cd $(UI_DIR) && npm run build

ui-test:
	cd $(UI_DIR) && npm test 2>/dev/null || true

# ─── QA ──────────────────────────────────────────────

qa:
	$(MAKE) core-test
	$(MAKE) dashboard-test
	$(MAKE) ui-build
