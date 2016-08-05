#!/bin/bash

set -e
LLCONF=/usr/local/bin/llconf
CLIENT_CERT="/client.cert.pem"

if [ ! -f "/initialized" ]
then
	while [ ! -f $CLIENT_CERT ]
	do
		echo "waiting for client cert..."
		sleep 20	
	done
	
	echo "install client certificate"
	$LLCONF server cert add --id client --path $CLIENT_CERT
	rm -f $CLIENT_CERT
	touch /initialized
fi

echo "startup server"
forego start 