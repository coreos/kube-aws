FROM golang:1.8.3

ARG KUBERNETES_VERSION=${KUBERNETES_VERSION:-v1.9.1}

RUN apt-get update && \
    apt-get install -y rsync && \
    go get -u github.com/jteeuwen/go-bindata/go-bindata && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /go

RUN git clone --depth 1 https://github.com/coreos/kubernetes -b $KUBERNETES_VERSION && \
    cd kubernetes && \
    git branch && \
    git checkout -b $KUBERNETES_VERSION && \
    git branch && \
    make all WHAT=cmd/kubectl && \
    find . -type f -executable -name kubectl && \
    ./_output/local/bin/linux/amd64/kubectl version --client && \
    make all WHAT=vendor/github.com/onsi/ginkgo/ginkgo && \
    make all WHAT=test/e2e/e2e.test && \
    rm -Rf .git

ENV KUBERNETES_PROVIDER skeleton
ENV KUBERNETES_CONFORMANCE_TEST Y

ADD conformance.sh /

WORKDIR /go/kubernetes

CMD /conformance.sh
