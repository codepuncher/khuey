# Makefile for KHuey Backend
# Handles CGo compilation with required flags

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean

# CGo flags for PipeWire
export CGO_CFLAGS_ALLOW=-fno-strict-overflow

# Build targets
BINARY_NAME=hue-sync
TEST_CAPTURE=test-capture
CMD_DIR=./cmd

.PHONY: all build test clean test-capture run-test

all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) $(CMD_DIR)/hue-sync

test-capture:
	$(GOBUILD) -o $(TEST_CAPTURE) $(CMD_DIR)/test-capture

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(TEST_CAPTURE)

# Run test-capture
run-test: test-capture
	./$(TEST_CAPTURE)
