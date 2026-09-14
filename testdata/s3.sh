#!/bin/bash

export AWS_ACCESS_KEY_ID=admin
export AWS_SECRET_ACCESS_KEY=secret

# Allocate a port.  Another process could bind to it before ours does,
# but that's very unlikely.
port=$(python3 -c "import socket; print(socket.create_server(('localhost', 0)).getsockname()[1])")

dir=$PWD/s3
mkdir $dir
RUSTFS_OBS_LOG_DIRECTORY=rustfs rustfs server $dir \
  --address localhost:$port --access-key $AWS_ACCESS_KEY_ID --secret-key $AWS_SECRET_ACCESS_KEY &
trap "kill -9 $!" EXIT

# Wait for server to accept a connection.
python3 <<EOF
import socket, time
start = time.time()
while True:
    try:
        socket.create_connection(('localhost', $port))
        break
    except ConnectionRefusedError:
        if time.time() - start > 5:
            raise
    time.sleep(0.1)
EOF

export AWS_ENDPOINT_URL_S3=http://localhost:$port
export AWS_REGION=does-not-matter
