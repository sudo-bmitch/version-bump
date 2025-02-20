ARG REGISTRY=docker.io
ARG ALPINE_VER=3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c
ARG GO_VER=1.24.0-alpine@sha256:2d40d4fc278dad38be0777d5e2a88a2c6dee51b0b29c97a764fc6c6a11ca893c

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
