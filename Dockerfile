ARG REGISTRY=docker.io
ARG ALPINE_VER=3.21.2@sha256:56fa17d2a7e7f168a043a2712e63aed1f8543aeafdcee47c58dcffe38ed51099
ARG GO_VER=1.23.6-alpine@sha256:2c49857f2295e89b23b28386e57e018a86620a8fede5003900f2d138ba9c4037

FROM ${REGISTRY}/library/golang:${GO_VER} AS build
RUN apk add --no-cache \
      ca-certificates \
      make
RUN adduser -D appuser
WORKDIR /src
COPY . /src/
RUN make bin/version-bump
USER appuser
CMD [ "bin/version-bump" ]

FROM scratch AS artifact
COPY --from=build /src/bin/version-bump /version-bump

FROM ${REGISTRY}/library/alpine:${ALPINE_VER} AS release-alpine
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build /src/bin/version-bump /usr/local/bin/version-bump
USER appuser
CMD [ "version-bump", "--help" ]
LABEL maintainer="" \
      org.opencontainers.image.authors="https://github.com/sudo-bmitch" \
      org.opencontainers.image.url="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.documentation="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.source="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="version-bump" \
      org.opencontainers.image.description=""

FROM scratch AS release-scratch
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build /src/bin/version-bump /version-bump
USER appuser
ENTRYPOINT [ "/version-bump" ]
CMD [ "--help" ]
LABEL maintainer="" \
      org.opencontainers.image.authors="https://github.com/sudo-bmitch" \
      org.opencontainers.image.url="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.documentation="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.source="https://github.com/sudo-bmitch/version-bump" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="version-bump" \
      org.opencontainers.image.description=""
