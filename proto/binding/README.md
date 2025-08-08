# binding proto

This is a proto file that defines binding tags for Sphere.

### Example

```protobuf
syntax = "proto3";

package shared.v1;

import "buf/validate/validate.proto";
import "google/api/annotations.proto";
import "sphere/binding/binding.proto";
import "sphere/errors/errors.proto";


enum TestEnum {
  TEST_ENUM_UNSPECIFIED = 0;
  TEST_ENUM_VALUE1 = 1;
  TEST_ENUM_VALUE2 = 2;
}

message RunTestRequest {
  string field_test1 = 1;
  int64 field_test2 = 2;
  string path_test1 = 3 [(sphere.binding.location) = BINDING_LOCATION_URI];
  int64 path_test2 = 4 [(sphere.binding.location) = BINDING_LOCATION_URI];
  string query_test1 = 5 [
    (buf.validate.field).required = true,
    (sphere.binding.location) = BINDING_LOCATION_QUERY
  ];
  int64 query_test2 = 6 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  repeated TestEnum enum_test1 = 7 [
    (sphere.binding.location) = BINDING_LOCATION_QUERY,
    (sphere.binding.tags) = "sphere:\"enum_test1\""
  ];
}
```