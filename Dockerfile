# Stage 1: build SPA
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: build Go binary
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /terraflow ./server/cmd/server

# Stage 3: minimal runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /terraflow /terraflow
COPY samples/ /samples/
ENTRYPOINT ["/terraflow"]
