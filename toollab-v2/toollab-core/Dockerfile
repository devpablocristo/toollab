FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY toollab-v2/go.mod ./
COPY toollab-v2/cmd ./cmd
COPY toollab-v2/internal ./internal

RUN go build -o /out/toollab-v2-api ./cmd/toollab-api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/toollab-v2-api /app/toollab-v2-api

EXPOSE 8090
CMD ["/app/toollab-v2-api"]
