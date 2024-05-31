# Go parameters
GOCMD=go
GOBUILD=${GOCMD} build
GOCLEAN=${GOCMD} clean
GOTEST=${GOCMD} test
GOGET=${GOCMD} get

# Binary
BINARY_PATH=./build-bin
BINARY_NAME=xrealxr

# Source files
SOURCES=$(wildcard *.go)

all: test build

build:
	mkdir -p ${BINARY_PATH}
	${GOBUILD} -o ${BINARY_PATH}/${BINARY_NAME} -v

test:
	${GOTEST} -v ./...

clean:
	${GOCLEAN}
	rm -rf ${BINARY_PATH}

run:
	$(GOBUILD) -o ${BINARY_PATH}/${BINARY_NAME} -v ./...
	${BINARY_PATH}/${BINARY_NAME} ${ARGS}

.PHONY: all build test clean run