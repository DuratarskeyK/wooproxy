#!/bin/bash

BASE_ADDR=http://api.privateproxy.me:3000/api/v1
SERVER_ID=<SERVER_ID>
API_KEY=<API_KEY>

cd /opt/wooproxy
./wooproxy -api_addr $BASE_ADDR -api_key $API_KEY &

while :
do
  	BASE_ADDR=$BASE_ADDR SERVER_ID=$SERVER_ID API_KEY=$API_KEY ./ipscript.sh
        sleep 300
done

