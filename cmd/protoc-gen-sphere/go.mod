module github.com/TBXark/sphere/cmd/protoc-gen-sphere

go 1.23.0

toolchain go1.24.4

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.6-20250625184727-c923a0c2a132.1
	github.com/TBXark/sphere/proto/binding v0.0.0-20250717091253-cbd097d15520
	google.golang.org/genproto/googleapis/api v0.0.0-20250715232539-7130f93afb79
	google.golang.org/protobuf v1.36.6
)
