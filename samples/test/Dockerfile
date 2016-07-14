FROM golang:1.6.2
MAINTAINER denkhaus <peristaltic@gmx.net>

RUN apt-get update -yy && \
  	apt-get install git
	
ADD client.cert.pem /client.cert.pem
RUN go get github.com/denkhaus/llconf
RUN llconf server cert add --id client --path /client.cert.pem

EXPOSE 9954

ENTRYPOINT ["llconf"]
CMD ["server" "run"]