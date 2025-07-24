module github.com/TBXark/sphere/cmd/protoc-gen-sphere

go 1.23.0

toolchain go1.24.4

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.6-20250717185734-6c6e0d3c608e.1
	github.com/TBXark/sphere/proto/binding v0.0.0-20250724085428-d8d45d5cdead
	google.golang.org/genproto/googleapis/api v0.0.0-20250721164621-a45f3dfb1074
	google.golang.org/protobuf v1.36.6
)
