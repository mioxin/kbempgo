//go:build tools

package kbv1

//go:generate go install github.com/envoyproxy/protoc-gen-validate@latest
//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//go:generate go install github.com/alta/protopatch/cmd/protoc-gen-go-patch@latest
//go:generate go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
//go:generate go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
//go:generate go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
//go:generate protoc  -I$GOPATH/pkg/mod/github.com/envoyproxy/protoc-gen-validate@v1.2.1/validate -I/usr/include -I. --proto_path=$GOPATH/pkg/mod/github.com/envoyproxy/protoc-gen-validate@v1.2.1 --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --validate_out=lang=go:. --validate_opt=paths=source_relative --grpc-gateway_out=logtostderr=true:. --grpc-gateway_opt=paths=source_relative stor.proto

import (
	// force dependency
	// _ "github.com/alta/protopatch/patch"

	// tools
	// _ "github.com/alta/protopatch/cmd/protoc-gen-go-patch"
	_ "github.com/envoyproxy/protoc-gen-validate"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"

	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
