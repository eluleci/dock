#!/bin/bash

# pull latest image
docker pull eluleci/dock

# remove stopped containers
for existingContainerId in $(docker ps  --filter="name=Dock" -q -a);do docker stop $existingContainerId && docker rm $existingContainerId;done

# run image
docker run --publish 80:1707 --name Dock --restart=always eluleci/dock
newContainerId=`docker ps  --filter="name=Dock" -q -a`

# copy the dock-config.json to the container
docker cp dock-config.json $newContainerId:/go/dock-config.json

# start image again
docker start $newContainerId