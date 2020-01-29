SHELL:=/bin/bash

APP_VERSION?=1.1.3

# build vars
BUILD_DATE:=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
REPOSITORY:=clobucks

#run vars
CONFIG:=$$(pwd)/test/config
TRAVIS:=$$(pwd)/test/travis

# go tools
PACKAGES:=$(shell go list ./... | grep -v '/vendor/')
VETARGS:=-asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -rangeloops -shift -structtags -unsafeptr

build:
	@echo ">>> Building $(REPOSITORY)/mgob:$(APP_VERSION)"
	@docker build \
	    --build-arg BUILD_DATE=$(BUILD_DATE) \
	    --build-arg VCS_REF=$(TRAVIS_COMMIT) \
	    --build-arg VERSION=$(APP_VERSION) \
	    -t $(REPOSITORY)/mgob:$(APP_VERSION) .

travis:
	@echo ">>> Building mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) image"
	@docker build \
	    --build-arg BUILD_DATE=$(BUILD_DATE) \
	    --build-arg VCS_REF=$(TRAVIS_COMMIT) \
	    --build-arg VERSION=$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) \
	    -t $(REPOSITORY)/mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) .

	@echo ">>> Starting mgob container"
	@docker run -d --net=host --name mgob \
	    --restart unless-stopped \
	    -v "$(TRAVIS):/config" \
	    -v "/tmp/ssh_host_rsa_key:/etc/ssh/ssh_host_rsa_key:ro" \
	    -v "/tmp/ssh_host_rsa_key.pub:/etc/ssh/ssh_host_rsa_key.pub:ro" \
        $(REPOSITORY)/mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) \
		-ConfigPath=/config \
		-StoragePath=/storage \
		-TmpPath=/tmp \
		-LogLevel=info

publish:
	@echo $(DOCKER_PASS) | docker login -u "$(DOCKER_USER)" --password-stdin
	@docker tag $(REPOSITORY)/mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) $(REPOSITORY)/mgob:edge
	@docker push $(REPOSITORY)/mgob:edge

release:
	@echo $(DOCKER_PASS) | docker login -u "$(DOCKER_USER)" --password-stdin
	@docker tag $(REPOSITORY)/mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) $(REPOSITORY)/mgob:$(APP_VERSION)
	@docker tag $(REPOSITORY)/mgob:$(APP_VERSION).$(TRAVIS_BUILD_NUMBER) $(REPOSITORY)/mgob:latest
	@docker push $(REPOSITORY)/mgob:$(APP_VERSION)
	@docker push $(REPOSITORY)/mgob:latest

run:
	@docker network create mgob || true
	@docker rm -f mgob-$(APP_VERSION) || true
	@echo ">>> Starting mgob container"
	docker run -dp 8090:8090 --net=mgob --name mgob-$(APP_VERSION) \
	    --restart unless-stopped \
	    -v "$(CONFIG):/config" \
	    -v /tmp/ssh_host_rsa_key.pub:/home/test/.ssh/keys/ssh_host_rsa_key.pub:ro \
		-v /tmp/ssh_host_rsa_key:/etc/ssh/ssh_host_rsa_key \
        $(REPOSITORY)/mgob:$(APP_VERSION) \
		-ConfigPath=/config \
		-StoragePath=/storage \
		-TmpPath=/tmp \
		-LogLevel=debug

backend:
	@docker network create mgob || true
	@docker run -dp 20022:22 --net=mgob --name mgob-sftp \
	    atmoz/sftp:alpine test:test:::backup
	@docker run -dp 20099:9000 --net=mgob --name mgob-s3 \
	    -e "MINIO_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE" \
	    -e "MINIO_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
	    minio/minio server /export
	@sleep 2
	@mc config host add local http://localhost:20099 \
	    AKIAIOSFODNN7EXAMPLE wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY S3v4
	@mc mb local/backup
	@ssh-keygen -b 4096 -t rsa -N "" -f /tmp/ssh_host_rsa_key -q
	@docker run -dp 20023:22 --net=mgob \
        -v /tmp/ssh_host_rsa_key.pub:/home/test/.ssh/keys/ssh_host_rsa_key.pub:ro \
        -v /tmp/ssh_host_rsa_key:/etc/ssh/ssh_host_rsa_key \
        --name test-sftp atmoz/sftp:alpine test::1001::backup

MONGO_PORT=27017
MONGO_REPLICA_PORT=27018
MONGO_CONTAINER=test-mongodb
MONGO_REPLICA_CONTAINER=test-replicaset-mongodb

mongo:
	@docker network create mgob || true
	@docker run -dp $(MONGO_PORT):27017 --net=mgob --name $(MONGO_CONTAINER) mongo:4.0.14
	@sleep 3
	@mongo test --eval 'db.test.insert({item: "item", val: "test" });'
	@docker run -dp $(MONGO_REPLICA_PORT):27017 --net=mgob --name $(MONGO_REPLICA_CONTAINER) mongo:4.0.14 mongod --replSet test-set
	@docker cp ./test/init_replica_set.js test-replicaset-mongodb:/init_replica_set.js
	@sleep 3
	@docker exec $(MONGO_REPLICA_CONTAINER) mongo localhost:27017/admin /init_replica_set.js
	@sleep 2
	@@mongo --port $(MONGO_REPLICA_PORT) test --eval 'db.test.insert({item: "item", val: "test" });'

cleanup:
	@docker rm -f test-mongodb test-sftp mgob-s3 mgob-sftp test-replicaset-mongodb mgob-$(APP_VERSION) || true
	@rm -rf /tmp/ssh_host_rsa_key /tmp/ssh_host_rsa_key.pub
	@docker network rm mgob

fmt:
	@echo ">>> Running go fmt $(PACKAGES)"
	@go fmt $(PACKAGES)

vet:
	@echo ">>> Running go vet $(VETARGS)"
	@go list ./... \
		| grep -v /vendor/ \
		| cut -d '/' -f 4- \
		| xargs -n1 \
			go tool vet $(VETARGS) ;\
	if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "go vet failed"; \
	fi

.PHONY: build
