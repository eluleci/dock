#!/bin/bash

# pull latest image
docker pull eluleci/dock

# remove stopped containers
for existingContainerId in $(docker ps  --filter="name=mentornity-api" -q -a);do docker stop $existingContainerId && docker rm $existingContainerId;done

# run image
docker run --publish 1707:1707 --name mentornity-api --restart=always eluleci/dock
newContainerId=`docker ps  --filter="name=mentornity-api" -q -a`

# copy the dock-config.json to the container
docker cp dock-config.json $newContainerId:/go/dock-config.json

# start image again
docker start $newContainerId