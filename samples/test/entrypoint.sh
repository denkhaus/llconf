#!/bin/bash

set -e

CLIENT_CERT="/client.cert.pem"


while [ ! -f $CLIENT_CERT ]
do
	echo "waiting for client cert..."
	sleep 20	
done

echo "install client certificate"
llconf server cert add --id client --path $CLIENT_CERT

echo "startup server"
llconf server run