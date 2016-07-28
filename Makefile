SHA 				= $(shell git rev-parse --short HEAD)
BUILD_VERSION 		:= $(shell date -u +%y-%m-%d-%H-%M-%S)
BUILD_TARGET		= bin/llconf
DOCKER_IMAGE		= denkhaus/llconf

all: build git-post

################################################################################
start-docker: build-docker start-docker

################################################################################
start-docker:		
	- docker rm -f llconf
	docker run -d --name llconf -p 9954:9954 $(DOCKER_IMAGE)
	docker cp ~/.llconf/cert/client.cert.pem llconf:/client.cert.pem
	sleep 25
	
	rm -f /tmp/server.cert.pem
	docker cp llconf:/root/.llconf/cert/server.cert.pem /tmp/server.cert.pem
	
	- llconf client cert rm --id docker
	llconf client cert add --id docker --path /tmp/server.cert.pem
	rm -f /tmp/server.cert.pem
	docker logs -f llconf

################################################################################
push-container:
	docker push $(DOCKER_IMAGE)

################################################################################
build-docker: git-pre git-post
	- docker rm -f llconf
	- docker rmi -f $(DOCKER_IMAGE)
	docker build -t $(DOCKER_IMAGE) docker/

################################################################################
git-pre:
	@echo "\n\n################# ---->  git prepare"
	- git add -A && git commit -am "$(BUILD_VERSION)"	

################################################################################
git-post:
	@echo "\n\n################# ---->  git push"
	git push origin master	
	
################################################################################
debug:
	docker run -it --rm --entrypoint /bin/bash $(DOCKER_IMAGE) 

################################################################################
watch:
	llconf client  -i ./test/input watch
	
################################################################################
run:
	llconf client  -i ./test/input run

################################################################################
build: git-pre
	@echo "\n\n################# ---->  build $(BUILD_TARGET)"
	@go build -o $(BUILD_TARGET) \
		-ldflags "-w -s \
		-X main.Revision=$(SHA) \
		-X main.AppVersion=$(BUILD_VERSION)"	
	@echo "\n\n################# ---->  deploy $(BUILD_TARGET)"
	@mv $(BUILD_TARGET) $(GOBIN)
	@echo "current build: $(shell llconf -v)"
	