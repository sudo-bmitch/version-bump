ARG REGISTRY=docker.io
ARG ALPINE_VER=3.20.2@sha256:0a4eaa0eecf5f8c050e5bba433f58c052be7587ee8af3e8b3910ef9ab5fbe9f5
ARG GO_VER=1.22.5-alpine@sha256:0d3653dd6f35159ec6e3d10263a42372f6f194c3dea0b35235d72aabde86486e

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
