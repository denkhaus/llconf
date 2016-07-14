
DOCKER_IMAGE		   := denkhaus/llconf

all: build run

################################################################################
run:
	- docker rm llconf
	docker run -it --name llconf -p 9954:9954 $(DOCKER_IMAGE) -H 0.0.0.0 server run

################################################################################
install-cert:	
	docker cp llconf:/root/.llconf/cert/server.cert.pem /tmp
	- llconf client cert rm --id docker
	llconf client cert add --id docker --path /tmp/server.cert.pem
	rm -f /tmp/server.cert.pem

################################################################################
push:
	docker push $(DOCKER_IMAGE)

################################################################################
build:
	docker build -t $(DOCKER_IMAGE) .

################################################################################
debug:
	docker run -it --rm --entrypoint /bin/bash $(DOCKER_IMAGE) 
