all: fettle test

SRC=$(shell find . -name '*.go')

fettle: $(SRC)
	go get github.com/stretchr/testify/assert
	go get github.com/hashicorp/consul/api
	go get github.com/google/uuid
	go get github.com/jinzhu/configor
	env CGO_ENABLED=0 go build
	@file fettle

.PHONY: test
test: fettle
	go test github.com/ahoka/fettle/server
