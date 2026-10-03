FROM node:22-bookworm-slim AS ui
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go web/fallback.html ./web/
COPY --from=ui /src/web/build ./web/build
RUN CGO_ENABLED=0 go build -trimpath -o /ledgerdrill ./cmd/ledgerdrill
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -r /var/lib/apt/lists/* && useradd -u 10001 -m ledgerdrill && mkdir /data && chown ledgerdrill:ledgerdrill /data
USER 10001
COPY --from=build /ledgerdrill /usr/local/bin/ledgerdrill
EXPOSE 8098
VOLUME /data
ENTRYPOINT ["ledgerdrill","-addr","0.0.0.0:8098","-data","/data"]
