##
## Stage 1: Build the Go plugin
##
FROM heroiclabs/nakama-pluginbuilder:3.21.1 AS builder

WORKDIR /backend

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Go plugin as a shared library
RUN go build -buildmode=plugin -trimpath -o /backend/nebula_server.so

##
## Stage 2: Run Nakama with the compiled plugin
##
FROM heroiclabs/nakama:3.21.1

# Copy the compiled plugin into Nakama's modules directory
COPY --from=builder /backend/nebula_server.so /nakama/data/modules/nebula_server.so

# Runtime config from env (production: Neon; local: docker-compose postgres)
COPY config/config.template.yml /nakama/templates/config.template.yml
COPY scripts/docker-entrypoint.sh /nakama/docker-entrypoint.sh
COPY scripts/render-config.pl /nakama/scripts/render-config.pl
RUN chmod +x /nakama/docker-entrypoint.sh /nakama/scripts/render-config.pl

ENTRYPOINT ["/nakama/docker-entrypoint.sh"]
