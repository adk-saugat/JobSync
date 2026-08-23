# Cloud Run image for jobsync-server
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /jobsync-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /jobsync-server /jobsync-server
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/jobsync-server"]
