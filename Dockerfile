########################
# INSTRUCTIONS
########################

# Build image:
# docker build -t eluleci/dock .

# Run container
# docker run --publish 1707:8080 --name Dock dock

# After running the container there will be an output like 'No 'dock-config.json' file found.' and container will exit.
# Copy configuration file
# docker cp dock-config.json CONTAINER_ID:/go/dock-config.json

# Start container again
# docker start CONTAINER_ID

########################

# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/eluleci/dock

# Get source code from github
# RUN git clone https://github.com/eluleci/dock.git /go/src/github.com/eluleci/dock

# Install all dependencies
RUN cd src/github.com/eluleci/dock; go get ./...

# Build the dock command inside the container.
RUN go install github.com/eluleci/dock

# Run the dock command by default when the container starts.
ENTRYPOINT /go/bin/dock

# Document that the service listens on port 8080
EXPOSE 8080