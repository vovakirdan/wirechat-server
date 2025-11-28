# syntax=docker/dockerfile:1.6

ARG GO_VERSION=1.25.2

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/wirechat-server ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/wirechat-server /wirechat-server
COPY --from=builder /app/config.example.yaml /config.example.yaml
USER nonroot:nonroot
ENTRYPOINT ["/wirechat-server"]
