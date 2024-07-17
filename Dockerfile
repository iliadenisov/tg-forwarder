ARG REPO_URL=docker.io
ARG IMAGE_BUILDER=golang:1.22.5-alpine3.20
ARG IMAGE_RUNNER=alpine:3.20

## --- Builder

FROM ${REPO_URL}/${IMAGE_BUILDER} AS builder
SHELL [ "/bin/sh", "-ec" ]
WORKDIR /build

ARG GOPROXY_URL
ENV GOPROXY=${GOPROXY_URL}
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apk add git openssh ; \
    echo -e "[url \"ssh://git@github.com/\"]\n\tinsteadOf = https://github.com/" > ~/.gitconfig

COPY go.mod go.sum ./
COPY cmd/ cmd/
COPY internal/ internal/

RUN go mod download ; \
    go mod tidy ; \
    go build -o artifacts/bin/forwarder cmd/forwarder/main.go

## --- Runner

FROM ${IMAGE_RUNNER}
LABEL stage=runner

ARG APP_GID=1337
ARG APP_UID=1337
ENV APP_HOME=/home/app \
    TARGET_USER=app \
    TARGET_GROUP=app

RUN getent group ${APP_GID} || addgroup --gid ${APP_GID} ${TARGET_GROUP} ; \
    adduser -u ${APP_UID} --ingroup ${TARGET_GROUP} -D -S -h ${APP_HOME} ${TARGET_USER}

WORKDIR ${APP_HOME}
USER ${TARGET_USER}

COPY --from=builder /build/artifacts/bin/forwarder /usr/bin/forwarder
COPY --chown=${TARGET_USER}:${TARGET_GROUP} entrypoint.sh ./

CMD [ "/bin/sh", "/home/app/entrypoint.sh", "/usr/bin/forwarder" ]
