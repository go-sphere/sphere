# protoc-gen-sphere-binding
`protoc-gen-sphere-binding` is a protoc plugin that generates Go struct tags for Sphere binding from `.proto` files. It is designed to inspect service definitions within your protobuf files and automatically generate corresponding Go struct tags based on a specified template.

```protobuf
message RunTestRequest {
  string field_test1 = 1;
  int64 field_test2 = 2;
  string path_test1 = 3 [(sphere.binding.binding_location) = BINDING_LOCATION_URI];
  int64 path_test2 = 4 [(sphere.binding.binding_location) = BINDING_LOCATION_URI];
  string query_test1 = 5 [
    (buf.validate.field).required = true,
    (sphere.binding.binding_location) = BINDING_LOCATION_QUERY
  ];
  int64 query_test2 = 6 [(sphere.binding.binding_location) = BINDING_LOCATION_QUERY];
}

```