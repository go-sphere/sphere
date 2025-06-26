# options proto

This is a proto file that defines extra options for rpc methods.

### Example

```protobuf
syntax = "proto3";

package bot.v1;

import "tbxark/options/options.proto";

message StartRequest {}

message StartResponse {}

service CounterService {
  rpc Start(StartRequest) returns (StartResponse) {
    option (tbxark.options.options) = {
      key: "bot",
      extra: [
        {
          key: "command",
          value: "start",
        }
      ]
    };
  }
}

```