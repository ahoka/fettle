all: fettle

SRC=$(shell find . -name '*.go')

fettle: $(SRC)
	go build

.PHONY: test
test:
	go test ./lib
