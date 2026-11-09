# Stage 1: build SPA
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: build Go binary
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /terraflow ./server/cmd/server

# Stage 3: runtime. Needs git: approving a changeset creates the review branch
# with the git CLI (internal/localrepo).
FROM alpine:3.20
RUN apk add --no-cache git ca-certificates \
    && git config --system --add safe.directory '*' \
    && adduser -D -u 65532 -h /home/terraflow terraflow \
    && mkdir -p /workspace && chown 65532:65532 /workspace
COPY --from=build /terraflow /terraflow
COPY samples/ /samples/
COPY catalog/ /catalog/
ENV TF_WORKSPACE_DIR=/workspace HOME=/home/terraflow
# Never root: the app only needs to write /workspace (and git needs a HOME).
USER 65532
ENTRYPOINT ["/terraflow"]
