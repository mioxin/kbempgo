
#all: generate
all:

generate: api db

api: always 
	go generate ./api/kbemp/v1/generate.go

db: always
# 	go generate ./pkg/slnaudit/generate.go

go-get:
	go get -t ./...
	go mod tidy

test:
	go test ./...

proto-fmt: $(shell find ./api -type f -name '*.proto')
	clang-format -i $?

vendor: always
	go mod vendor

.PHONY: generate

always:
	@true
