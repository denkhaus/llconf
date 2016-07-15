
DOCKER_IMAGE		   := denkhaus/llconf

all: build run

################################################################################
run:		
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
push:
	docker push $(DOCKER_IMAGE)

################################################################################
build: git
	- docker rm -f llconf
	- docker rmi -f $(DOCKER_IMAGE)
	docker build -t $(DOCKER_IMAGE) docker/

################################################################################
git:
	go install
	git add -A 
	git commit -am "proceed"
	git push
	
################################################################################
debug:
	docker run -it --rm --entrypoint /bin/bash $(DOCKER_IMAGE) 