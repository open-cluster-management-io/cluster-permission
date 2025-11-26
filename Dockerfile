FROM registry.ci.openshift.org/stolostron/builder:go1.24-linux AS builder

WORKDIR /go/src/github.com/open-cluster-management-io/cluster-permission
COPY . .
RUN make -f Makefile build

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

RUN microdnf update -y && \
     microdnf clean all

ENV OPERATOR=/usr/local/bin/cluster-permission \
    USER_UID=1001 \
    USER_NAME=cluster-permission

# install operator binary
COPY --from=builder /go/src/github.com/open-cluster-management-io/cluster-permission/bin/cluster-permission /usr/local/bin/cluster-permission

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
