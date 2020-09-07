VERSION?=latest
IMAGE_REPO=quay.io/powercloud
IMAGE_NAME=build-bot
ARCH?=amd64

ALL_ARCH = amd64 ppc64le

#all-pull: $(addprefix sub-pull-,$(POD_UTILITIES))
#
#sub-pull-%:
#	docker pull ${UPSTREAM_REPO}/$*:${VERSION}

all-push: $(addprefix sub-push-,$(ALL_ARCH))

sub-push-%:
	docker push ${IMAGE_REPO}/${IMAGE_NAME}:$*-${VERSION}

all-build: $(addprefix sub-build-,$(ALL_ARCH))

sub-build-%:
	docker build --build-arg ARCH=$* -t ${IMAGE_REPO}/${IMAGE_NAME}:$*-${VERSION} .

export DOCKER_CLI_EXPERIMENTAL = enabled
push-manifest:
	docker manifest create --amend $(IMAGE_REPO)/$(IMAGE_NAME):$(VERSION) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(IMAGE_REPO)/$(IMAGE_NAME):&\-$(VERSION)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${IMAGE_REPO}/${IMAGE_NAME}:${VERSION} ${IMAGE_REPO}/${IMAGE_NAME}:$${arch}-${VERSION}; done
	docker manifest push --purge ${IMAGE_REPO}/${IMAGE_NAME}:${VERSION}
