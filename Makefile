# Makefile for nbartookt

# Go parameters
#  GOCMD=GOOS=linux GOARCH=amd64 go
#  GOBUILD=CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo
ifndef GOCMD
    GOCMD=go
endif
ifndef GOBUILD
    GOBUILD=$(GOCMD) build
endif
GOCLEAN=$(GOCMD) clean
GOINSTALL=$(GOCMD) install
GOTEST=$(GOCMD) test
GODEP=$(GOTEST) -i
GOFMT=gofmt -w
BINARY_NAME=wukong-eos

all: test
test:
	$(GOTEST) -v -cover=true -coverprofile=./sdk.cover ./...
	go tool cover -html=./sdk.cover -o ./sdk.html
fmt:
	find ./ -name "*.go" | xargs gofmt -w
