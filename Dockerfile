ARG BASE_IMAGE

FROM --platform=$BUILDPLATFORM golang:1.22.5-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH

ARG VERSION_PACKAGE=github.com/akuity/kargo-render/internal/version
ARG CGO_ENABLED=0

WORKDIR /kargo-render
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

ARG VERSION
ARG GIT_COMMIT
ARG GIT_TREE_STATE

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -o bin/credential-helper \
      ./cmd/credential-helper

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags "-w -X ${VERSION_PACKAGE}.version=${VERSION} -X ${VERSION_PACKAGE}.buildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X ${VERSION_PACKAGE}.gitCommit=${GIT_COMMIT} -X ${VERSION_PACKAGE}.gitTreeState=${GIT_TREE_STATE}" \
      -o bin/kargo-render \
      ./cmd/kargo-render \
    && bin/kargo-render version

FROM ${BASE_IMAGE}:latest-${TARGETARCH} AS final

COPY --from=builder /kargo-render/bin/ /usr/local/bin/

# Ensure that the XDG_*_HOME environment variables are set to a directory
# that is writable by the nonroot user. This is necessary because otherwise
# Helm fails to write cache files and is unable to download indexes and
# chart (dependencies).
ENV XDG_CONFIG_HOME=/tmp/.config
ENV XDG_CACHE_HOME=/tmp/.cache
ENV XDG_DATA_HOME=/tmp/.local/share

ENTRYPOINT ["/usr/local/bin/kargo-render"]
