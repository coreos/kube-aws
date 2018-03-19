KUBERNETES_VERSION ?= v1.9.1
DOCKER_REPO ?=
DOCKER_TAG ?= $(DOCKER_REPO)kube-e2e:$(KUBERNETES_VERSION)
DOCKER_TAG_SANITIZED ?= $(shell echo $(DOCKER_TAG) | sed -e 's/+/_/')
KUBE_AWS_ASSETS ?= /missing-kube-aws-assets
SSH_PRIVATE_KEY ?=
SSH_CMD ?= ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i $(SSH_PRIVATE_KEY)
MASTER_HOST ?=
FOCUS ?= \[Conformance\]

.PHONY: build
build:
	docker build --build-arg KUBERNETES_VERSION=$(KUBERNETES_VERSION) -t $(DOCKER_TAG_SANITIZED) .

.PHONY: publish
publish:
	docker push $(DOCKER_TAG_SANITIZED)

.PHONY: run-locally
run-locally:
	docker run --rm -v $(KUBE_AWS_ASSETS):/kube $(DOCKER_TAG_SANITIZED)

.PHONY: ssh
ssh:
	ssh -i $(SSH_PRIVATE_KEY) core@$(MASTER_HOST)

# example usage:
#   FOCUS='\[HPA\].*\[Conformance\]' DOCKER_REPO=quay.io/mumoshu/ MASTER_HOST=**.**.**.** SSH_PRIVATE_KEY=path/to/key KUBE_AWS_ASSETS=path/to/where/kube-aws/run sh -c 'make build && make publish && make run-remotely'
#
#   FOCUS:
#     a regexp to selectively run e2e tests. Default: \[Conformance\]
#     see https://github.com/kubernetes/kubernetes/blob/master/docs/devel/e2e-tests.md#conformance-tests for more examples and description
.PHONY: run-remotely
run-remotely:
	rsync -a -e '$(SSH_CMD)' $(KUBE_AWS_ASSETS)/ core@$(MASTER_HOST):/home/core/kube
	$(SSH_CMD) core@$(MASTER_HOST) 'sh -c "docker pull $(DOCKER_TAG_SANITIZED) && container_id=$$(docker run -d -v /home/core/kube:/kube -e FOCUS='$(FOCUS)' $(DOCKER_TAG_SANITIZED)) && echo \$$container_id && docker logs -f \$$container_id"'

.PHONY: get-log
get-log:
	full_container_id=$$($(SSH_CMD) core@$(MASTER_HOST) 'docker ps -a --no-trunc | grep $(DOCKER_TAG_SANITIZED) | tee /dev/stderr | head -n 1 | awk "{ print \$$1 }"') && \
	rsync --rsync-path="sudo rsync" -e '$(SSH_CMD)' core@$(MASTER_HOST):/var/lib/docker/containers/$${full_container_id}/$${full_container_id}-json.log .

.PHONY: show-log
show-log:
	$(SSH_CMD) core@$(MASTER_HOST) 'sh -c "container_id=$$(docker ps -a | grep $(DOCKER_TAG_SANITIZED) | head -n 1 | cut -d" " -f 1); docker logs -f \$$container_id"'

.PHONY: sh
sh:
	docker run -it --rm -v $(KUBE_AWS_ASSETS):/kube $(DOCKER_TAG_SANITIZED) sh
