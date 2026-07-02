#!/bin/bash

TAG=SNAPSHOT-3b1b4219b8cfabd3a62e7da67ece0d6069887418-arm64
OUTPUT=sablier-$TAG.tgz

docker image save sablierapp/sablier:$TAG -o $OUTPUT
docker -c raspberrypi4 image load -i $OUTPUT