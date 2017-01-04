all: fettle

fettle:
	go build

.PHONY: test
test:
	go test ./lib
