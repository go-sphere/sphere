module github.com/TBXark/sphere/cmd/protoc-gen-route

go 1.23.2

replace github.com/TBXark/sphere/internal/protogo => ../../internal/protogo

require (
	github.com/TBXark/sphere/internal/protogo v0.0.0-00010101000000-000000000000
	github.com/tbxark/options-proto/go v0.0.0-20241107032846-d46ef06aa5e1
	google.golang.org/protobuf v1.36.6
)
