# Cloud Run image for jobsync-server (API + setup web UI)

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist/. ./internal/cloud/server/webdist/
RUN CGO_ENABLED=0 go build -o /jobsync-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /jobsync-server /jobsync-server
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/jobsync-server"]
