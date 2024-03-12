ARG REGISTRY=docker.io
ARG ALPINE_VER=3.19.1@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b
ARG GO_VER=1.22.1-alpine@sha256:fc5e5848529786cf1136563452b33d713d5c60b2c787f6b2a077fa6eeefd9114

FROM ${REGISTRY}/library/golang:${GO_VER} as build
RUN apk add --no-cache \
      ca-certificates \
      make
RUN adduser -D appuser
WORKDIR /src
COPY . /src/
RUN make bin/version-bump
USER appuser
CMD [ "bin/version-bump" ]

FROM scratch as artifact
COPY --from=build /src/bin/version-bump /version-bump

FROM ${REGISTRY}/library/alpine:${ALPINE_VER} as release-alpine
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

FROM scratch as release-scratch
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
