FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache gcc musl-dev

COPY toollab-dashboard/go.mod toollab-dashboard/go.sum ./toollab-dashboard/
RUN cd toollab-dashboard && go mod download

COPY toollab-dashboard ./toollab-dashboard
RUN cd toollab-dashboard && CGO_ENABLED=1 GOOS=linux go build -o /out/toollab-dashboard ./cmd/api

COPY toollab-core/go.mod toollab-core/go.sum ./toollab-core/
RUN cd toollab-core && go mod download

COPY toollab-core ./toollab-core
RUN cd toollab-core && CGO_ENABLED=0 GOOS=linux go build -o /out/toollab ./cmd/toollab

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app

COPY --from=builder /out/toollab-dashboard /app/toollab-dashboard
COPY --from=builder /out/toollab /app/toollab
COPY toollab-dashboard/migrations /app/migrations

# toollab-core needs schemas/ and .env relative to its cwd
COPY schemas /app/core/schemas
COPY toollab-core/.env* /app/core/

RUN mkdir -p /app/data /app/core/scenarios /app/core/golden_runs && chown -R app:app /app

USER app
EXPOSE 8090

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8090/healthz >/dev/null 2>&1 || exit 1

CMD ["/app/toollab-dashboard"]
