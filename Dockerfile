FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

ARG TARGETARCH

WORKDIR /workspace

ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-w -s" -o config-reloader-sidecar .

# UPX compression
FROM --platform=$BUILDPLATFORM gruebel/upx:latest AS upx

COPY --from=builder /workspace/config-reloader-sidecar .

RUN upx --best --lzma /config-reloader-sidecar

# Runtime

FROM --platform=$TARGETPLATFORM gcr.io/distroless/static-debian12:latest

COPY --from=upx /config-reloader-sidecar .

CMD ["/config-reloader-sidecar"]
