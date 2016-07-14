FROM golang:1.6.2
MAINTAINER denkhaus <peristaltic@gmx.net>

RUN apt-get update -yy && apt-get install git
	
RUN go get github.com/denkhaus/llconf
ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 9954
ENTRYPOINT ["/entrypoint.sh"]