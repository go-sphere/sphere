module github.com/TBXark/sphere/cmd/protoc-gen-sphere

go 1.23.0

toolchain go1.24.4

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.7-20250717185734-6c6e0d3c608e.1
	github.com/TBXark/sphere/proto/binding v0.0.0-20250811072740-1608e3852053
	google.golang.org/genproto/googleapis/api v0.0.0-20250804133106-a7a43d27e69b
	google.golang.org/protobuf v1.36.7
)
