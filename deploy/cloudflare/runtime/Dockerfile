ARG NODE_VERSION=26
ARG TRACEWAY_REVISION=dev

FROM --platform=linux/amd64 node:${NODE_VERSION}-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ENV CLOUD_MODE=true
RUN npm run build

FROM --platform=linux/amd64 golang:1.26-alpine AS backend-builder

WORKDIR /app/backend
RUN apk add --no-cache git
COPY backend/ ./
COPY cli/ /app/cli/
COPY --from=frontend-builder /app/frontend/build ./static/frontend/
RUN go mod edit -dropreplace=go.tracewayapp.com -dropreplace=go.tracewayapp.com/tracewaygin
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -tags cloudflare -ldflags="-s -w" -o /traceway ./cmd/traceway

FROM alpine:3.20

LABEL org.opencontainers.image.revision="${TRACEWAY_REVISION}"
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend-builder /traceway /usr/local/bin/traceway
USER nobody
WORKDIR /app
ENV GIN_MODE=release PORTS=8082 DB_TYPE=sqlite STORAGE_TYPE=s3 CLOUD_MODE=true TRUSTED_PROXY_HEADER=CF-Connecting-IP
EXPOSE 8082
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 CMD wget -qO- http://localhost:8082/health || exit 1
ENTRYPOINT ["/usr/local/bin/traceway"]
