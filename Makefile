# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Binary name
BINARY_NAME=xreallightxr

# Source files
SOURCES=$(wildcard *.go)

all: test build

build:
	$(GOBUILD) -o /tmp/$(BINARY_NAME) -v

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run:
	$(GOBUILD) -o /tmp/$(BINARY_NAME) -v ./...
	/tmp/$(BINARY_NAME) $(ARGS)

.PHONY: all build test clean run