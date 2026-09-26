# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/myscoutee-mcp ./cmd/myscoutee-mcp

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/myscoutee-mcp /myscoutee-mcp
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/myscoutee-mcp"]
