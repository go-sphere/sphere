module github.com/TBXark/sphere/cmd/protoc-gen-sphere

go 1.23.0

toolchain go1.24.4

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.6-20250625184727-c923a0c2a132.1
	github.com/TBXark/sphere/internal/protogo v0.0.0-20250626085116-d100979d0d20
	github.com/TBXark/sphere/internal/tags v0.0.0-20250626085116-d100979d0d20
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822
	google.golang.org/protobuf v1.36.6
)
