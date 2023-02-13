ARG GO_VERSION=1.20
ARG TARGET_UID=9999
ARG TARGET_GID=9999

# Base image with go compiler and tested source code
FROM --platform=$BUILDPLATFORM docker.io/library/golang:${GO_VERSION} as source

ARG TARGET_UID TARGET_GID

# Don't run the compiler as root
RUN for dir in /.cache "/apps"; do \
         mkdir -p "${dir}"; \
         chown "${TARGET_UID}:${TARGET_GID}" "${dir}"; \
    done
USER "${TARGET_UID}:${TARGET_GID}"
WORKDIR /usr/src/app

# Fetch dependencies first
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Fetch and test the source code
COPY --chown=${TARGET_UID}:${TARGET_GID} . .
RUN go vet -v ./...
RUN go test -v ./...

# Build binary
FROM source AS binaries
ARG APP_NAME TARGET_UID TARGET_GID TARGETOS TARGETARCH

ENV CGO_ENABLED=0
RUN mkdir -p "/apps/${APP_NAME}/"
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -v \
        -ldflags="-extldflags=-static" \
        -o "/apps/${APP_NAME}/${APP_NAME}" \
        "./cmd/${APP_NAME}"
RUN ln -s "/${APP_NAME}" "/apps/${APP_NAME}/app"


# Prepare target filesystems
FROM alpine:latest as filesystem
ARG TARGET_UID TARGET_GID APP_NAME
COPY --from=binaries --chown=0:0 /apps/ /filesystem/
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /filesystem/web_proxy/etc/ssl/certs/
RUN mkdir -p /filesystem/static_datastore_builder/data \
 && chown "$TARGET_UID:$TARGET_GID" /filesystem/static_datastore_builder/data
RUN find /filesystem/${APP_NAME}

FROM scratch
ARG APP_NAME TARGET_UID TARGET_GID
COPY --from=filesystem /filesystem/${APP_NAME} /
USER ${TARGET_UID}:${TARGET_GID}
CMD [ "/${APP_NAME}" ]
