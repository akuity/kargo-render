FROM --platform=$BUILDPLATFORM golang:1.22.1-bookworm as builder

ARG TARGETOS
ARG TARGETARCH

ARG HELM_VERSION=v3.12.3
RUN curl -L -o /tmp/helm.tar.gz \
      https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz \
    && tar xvfz /tmp/helm.tar.gz -C /usr/local/bin --strip-components 1

ARG KUSTOMIZE_VERSION=v5.1.1
RUN curl -L -o /tmp/kustomize.tar.gz \
      https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_${TARGETARCH}.tar.gz \
    && tar xvfz /tmp/kustomize.tar.gz -C /usr/local/bin

ARG YTT_VERSION=v0.46.0
RUN curl -L -o /usr/local/bin/ytt \
      https://github.com/vmware-tanzu/carvel-ytt/releases/download/${YTT_VERSION}/ytt-linux-${TARGETARCH} \
      && chmod 755 /usr/local/bin/ytt

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
      -ldflags "-w -X ${VERSION_PACKAGE}.version=${VERSION} -X ${VERSION_PACKAGE}.buildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X ${VERSION_PACKAGE}.gitCommit=${GIT_COMMIT} -X ${VERSION_PACKAGE}.gitTreeState=${GIT_TREE_STATE}" \
      -o bin/kargo-render \
      ./cmd \
    && bin/kargo-render version \
    && cd bin \
    && ln -s kargo-render kargo-render-action

FROM alpine:3.15.4 as final

RUN apk update \
    && apk add git openssh-client \
    && addgroup -S -g 65532 nonroot \
    && adduser -S -D -H -u 65532 -g nonroot -G nonroot nonroot

COPY --from=builder /usr/local/bin/helm /usr/local/bin/
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/
COPY --from=builder /usr/local/bin/ytt /usr/local/bin/
COPY --from=builder /kargo-render/bin/ /usr/local/bin/

USER nonroot

# Ensure that the XDG_*_HOME environment variables are set to a directory
# that is writable by the nonroot user. This is necessary because otherwise
# Helm fails to write cache files and is unable to download indexes and
# chart (dependencies).
ENV XDG_CONFIG_HOME=/tmp/.config
ENV XDG_CACHE_HOME=/tmp/.cache
ENV XDG_DATA_HOME=/tmp/.local/share

CMD ["/usr/local/bin/kargo-render"]
