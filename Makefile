CALICO_BUILD?=calico/go-build
PACKAGE_NAME?=kubernetes-incubator/kube-aws
LOCAL_USER_ID?=$(shell id -u $$USER)

.PHONY: build
build:
	./build

.PHONY: format
format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: test
test: build
	./make/test

.PHONY: test-with-cover
test-with-cover: build
	./make/test with-cover

.PHONY: docs-dependencies
docs-dependencies:
	if ! which gitbook; then npm install -g gitbook-cli; fi
	if ! which gh-pages; then npm install -g gh-pages; fi
	gitbook install

.PHONY: generate-docs
generate-docs: docs-dependencies
	gitbook build

.PHONY: serve-docs
serve-docs: docs-dependencies
	gitbook serve

# For publishing to the `git remote` named `mumoshu` linked to the repo url `git@github.com:mumoshu/kube-aws.git`, the env vars should be:
#   REPO=mumoshu/kube-aws REMOTE=mumoshu make publish-docs
# For publishing to the `git remote` named `origin` linked to the repo url `git@github.com:kubernetes-incubator/kube-aws.git`, the env vars should be:
#   REPO=kubernetes-incubator/kube-aws REMOTE=origin make publish-docs
# Or just:
#   make publish-docs

.PHONY: publish-docs
publish-docs: REPO ?= kubernetes-incubator/kube-aws
publish-docs: REMOTE ?= origin
publish-docs: generate-docs
	NODE_DEBUG=gh-pages gh-pages -d _book -r git@github.com:$(REPO).git -o $(REMOTE)

.PHONY: relnote
relnote:
	go get golang.org/x/oauth2
	go get golang.org/x/net/context
	go get github.com/google/go-github/github
	go run hack/relnote.go

.PHONY: merged-branches
merged-branches:
	@git branch --merged | egrep -v "(^\*|master|v0.)"
	@bash -c "(echo 'pipe this into \`| xargs git branch -d\`' to delete) 1>&2"
