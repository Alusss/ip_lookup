# syntax=docker/dockerfile:1.7
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ip-lookup ./backend

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/ip-lookup /usr/local/bin/ip-lookup
COPY backend/config.yaml /etc/ip-lookup/config.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/ip-lookup", "-config", "/etc/ip-lookup/config.yaml"]
