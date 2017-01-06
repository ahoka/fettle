all: fettle

SRC=$(shell find . -name '*.go')

fettle: $(SRC)
	get get github.com/hashicorp/consul/api
	go get github.com/google/uuid
	go build

.PHONY: test
test:
	go test ./lib
