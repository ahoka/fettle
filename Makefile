all: fettle

SRC=$(shell find . -name '*.go')

fettle: $(SRC)
	go get github.com/stretchr/testify/assert
	go get github.com/hashicorp/consul/api
	go get github.com/google/uuid
	go build

.PHONY: test
test:
	go test github.com/ahoka/fettle/server
