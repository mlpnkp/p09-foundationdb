FROM golang:1.26 AS build
ARG TARGETARCH
ARG FDB_VERSION=8.0.0
RUN set -eux; \
    case "$TARGETARCH" in arm64) arch=aarch64 ;; amd64) arch=amd64 ;; *) echo "unsupported $TARGETARCH"; exit 1 ;; esac; \
    deb="foundationdb-clients_${FDB_VERSION}-1_${arch}.deb"; \
    url="https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/${deb}"; \
    curl -fsSL -o "/tmp/${deb}" "$url"; \
    curl -fsSL -o "/tmp/${deb}.sha256" "${url}.sha256"; \
    echo "$(cut -d' ' -f1 "/tmp/${deb}.sha256")  /tmp/${deb}" | sha256sum -c -; \
    dpkg -i "/tmp/${deb}"; \
    rm "/tmp/${deb}" "/tmp/${deb}.sha256"
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
RUN go vet ./... && go build -o /out/loadgen ./cmd/loadgen && go build -o /out/summary ./cmd/summary

FROM debian:trixie-slim
COPY --from=build /usr/lib/libfdb_c.so /usr/lib/libfdb_c.so
COPY --from=build /out/loadgen /out/summary /usr/local/bin/
