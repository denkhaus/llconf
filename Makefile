SHA 				= $(shell git rev-parse --short HEAD)
HOSTNAME			= $(shell hostname)
BUILD_VERSION 		= $(shell date -u +%y-%m-%d_%H\:%M\:%S)

BUILD_TARGET_AMD64	= bin/llconf-linux-amd64
BUILD_TARGET_ARM64	= bin/llconf-linux-arm64
BUILD_TARGET_ARM	= bin/llconf-linux-arm

CURRENT_VERSION     = $(shell llconf -v)
CURRENT_REVISION    = $(shell llconf --revision)

LIB_REPO_PATH		= ~/.llconf/lib
DOCKER_IMAGE		= denkhaus/llconf

################################################################################
define build_release
	@echo "\n\n################# ----> build release $(CURRENT_REVISION) for $1/$2 -> target $3"
	@rm -f $3
	@env GOOS=$1 GOARCH=$2 go build -o $3 \
		-ldflags "-w -s \
		-X main.Revision=$(SHA) \
		-X main.AppVersion=$(BUILD_VERSION)"			
endef

################################################################################
define upload_release
	@echo "\n\n################# ----> upload release $(CURRENT_REVISION) for $1/$2 -> target $3"	
	github-release upload \
	    -u denkhaus \
	    -r llconf \
	    -t $(SHA) \
	    -n "llconf-$(SHA)-$1-$2" \
    	-f $3
endef

################################################################################
all: build 

################################################################################
release: build git-post wait push-release update-lib

################################################################################
run-docker: build-docker		
	- docker rm -f llconf
	docker run -h ${HOSTNAME} -d --name llconf -p 9954:9954 $(DOCKER_IMAGE)
	docker cp ~/.llconf/cert/client.cert.pem llconf:/client.cert.pem
	# wait for entrypoint to startup
	sleep 25
	
	rm -f /tmp/server.cert.pem
	docker cp llconf:/root/.llconf/cert/server.cert.pem /tmp/server.cert.pem
	
	- llconf client cert rm --id docker
	llconf client cert add --id docker --path /tmp/server.cert.pem
	rm -f /tmp/server.cert.pem
	docker logs -f llconf

################################################################################
start-docker:
	@docker start -a llconf
	
################################################################################
push-docker:
	docker push $(DOCKER_IMAGE)

################################################################################
build-docker: 	
	- docker rmi -f $(DOCKER_IMAGE)
	docker build --build-arg REVISION=$(SHA) -t $(DOCKER_IMAGE) docker/

################################################################################
git-pre:
	@echo "\n\n################# ---->  git prepare llconf"
	- git add -A && git commit -am "$(BUILD_VERSION)"	

################################################################################
git-post: delete-old-releases	
	@echo "\n################# ---->  remove remote and local tags"	
	@git tag --list | xargs git push --delete origin	
	@git tag --list | xargs git tag -d
	@echo "\n################# ---->  git push $(CURRENT_VERSION)"	
	@git tag $(SHA)
	@git push --tags origin master	
	
################################################################################
debug:
	docker run -it --rm --entrypoint /bin/bash $(DOCKER_IMAGE) 

################################################################################
build: git-pre
	$(call build_release,linux,arm,$(BUILD_TARGET_ARM))
	$(call build_release,linux,arm64,$(BUILD_TARGET_ARM64))
	$(call build_release,linux,amd64,$(BUILD_TARGET_AMD64))
	@echo "\n################# ---->  deploy $(BUILD_TARGET_AMD64)"
	@cp $(BUILD_TARGET_AMD64) $(GOBIN)/llconf
	
################################################################################
update-lib:	
	@echo "\n################# ---->  update lib revision to $(CURRENT_REVISION)"
	@cd $(LIB_REPO_PATH) && \
	echo $(CURRENT_REVISION) > .llconf_rev && \
	git add -A && git commit -am "update current rev: $(CURRENT_REVISION)" && \
	git push origin master	

################################################################################
push-release: 
	@echo "\n################# ---->  create release for $(CURRENT_REVISION)"
	@github-release release \
    -u denkhaus \
    -r llconf \
    -t $(SHA) \
    -n "$(CURRENT_VERSION)" \
    -d "llconf - configuration managment solution"
	
	$(call upload_release,linux,arm,$(BUILD_TARGET_ARM))
	$(call upload_release,linux,arm64,$(BUILD_TARGET_ARM64))	
	$(call upload_release,linux,amd64,$(BUILD_TARGET_AMD64))	
    
################################################################################
delete-old-releases:
	- git tag --list | xargs github-release delete -u denkhaus -r llconf -t 
    
################################################################################
wait:
	@echo "\n################# ---->  wait until github recognizes new tag"
	@ sleep 10




	