FROM ubuntu:14.04
MAINTAINER denkhaus <peristaltic@gmx.net>

RUN apt-get update -y && \
	DEBIAN_FRONTEND=noninteractive && \
	apt-get install -y git curl

#install forego
ADD Procfile /Procfile
RUN curl -L https://bin.equinox.io/c/ekMN3bCZFUn/forego-stable-linux-amd64.tgz | tar -xzf - -C /usr/local/bin  	

#install llconf
ARG REVISION
RUN curl -L  https://github.com/denkhaus/llconf/releases/download/${REVISION}/llconf-${REVISION} -o /usr/local/bin/llconf
RUN chmod u+x /usr/local/bin/llconf
ADD entrypoint.sh /entrypoint.sh
RUN chmod u+x /entrypoint.sh	

EXPOSE 9954
ENTRYPOINT ["/entrypoint.sh"]