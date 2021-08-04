REGISTRY=louhwz
ARCHS?=amd64
DEV_TAG?=dev
RELEASE_TAG?=release

.PHONY: build-dev-images release

build-dev-images:
	@for arch in ${ARCHS} ; do \
    	docker build -t ${REGISTRY}/rama:${DEV_TAG}-$$arch -f Dockerfile.$$arch ./; \
    done

release:
	@for arch in ${ARCHS} ; do \
		docker build -t ${REGISTRY}/rama:${RELEASE_TAG}-$$arch -f Dockerfile.$$arch ./; \
	done

code-gen:
	cd hack && chmod u+x ./update-codegen.sh &&controller.go:188 ./update-codegen.sh

push: build-dev-images
	docker push ${REGISTRY}/rama:${DEV_TAG}-amd64