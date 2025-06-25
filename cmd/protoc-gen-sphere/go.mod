module github.com/TBXark/sphere/cmd/protoc-gen-sphere

go 1.23.2

replace github.com/TBXark/sphere/internal/tags => ../../internal/tags

replace github.com/TBXark/sphere/internal/protogo => ../../internal/protogo

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.6-20250613105001-9f2d3c737feb.1
	github.com/TBXark/sphere/internal/protogo v0.0.0-00010101000000-000000000000
	github.com/TBXark/sphere/internal/tags v0.0.0-00010101000000-000000000000
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822
	google.golang.org/protobuf v1.36.6
)
