FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/toollab-dashboard ./cmd/toollab-dashboard

FROM golang:1.25-alpine
RUN apk add --no-cache ca-certificates gcc musl-dev
WORKDIR /app
COPY --from=builder /bin/toollab-dashboard .
RUN mkdir -p data
EXPOSE 8090
CMD ["./toollab-dashboard"]
