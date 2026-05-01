FROM golang:1.26.1-alpine AS builder

RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /bin/api .
RUN mkdir -p data
EXPOSE 8090
CMD ["./api"]
