FROM golang:1.13.3-stretch

COPY . /go/src/github.com/movetokube/postgres-operator
WORKDIR /go/src/github.com/movetokube/postgres-operator/cmd/manager

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/postgres-operator


FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/postgres-operator \
    USER_UID=1001 \
    USER_NAME=postgres-operator

# install operator binary
COPY --from=0 /usr/local/bin/postgres-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

