FROM  golang:1.23
MAINTAINER Florian Bergmann <Bergmann.F@gmail.com>

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY *.go ./
COPY telegram/ ./telegram
COPY nextcloud ./nextcloud

RUN ls -lah
# Build
RUN GOOS=linux go build -o /rpgreminder

# Optional:
# To bind to a TCP port, runtime parameters must be supplied to the docker command.
# But we can document in the Dockerfile what ports
# the application is going to listen on by default.
# https://docs.docker.com/reference/dockerfile/#expose
EXPOSE 8080

# Run
CMD ["/rpgreminder"]
