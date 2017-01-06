all: fettle

SRC=$(shell find . -name '*.go')

fettle: $(SRC)
	go get github.com/google/uuid
	go build

.PHONY: test
test:
	go test ./lib
