#!/bin/bash

set -e

CLIENT_CERT="/client.cert.pem"

if [ ! -f "/initialized" ]
then
	while [ ! -f $CLIENT_CERT ]
	do
		echo "waiting for client cert..."
		sleep 20	
	done
	
	echo "install client certificate"
	llconf server cert add --id client --path $CLIENT_CERT
	rm -f $CLIENT_CERT
	touch /initialized
fi

echo "startup server"
llconf -H 0.0.0.0 server run