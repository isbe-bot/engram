# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/engramd ./cmd/engramd \
 && CGO_ENABLED=0 go build -o /out/engramctl ./cmd/engramctl

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/engramd /usr/local/bin/engramd
COPY --from=build /out/engramctl /usr/local/bin/engramctl
COPY configs/example.yaml /etc/engram/engram.yaml
EXPOSE 8787
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/engramd", "--config", "/etc/engram/engram.yaml"]
