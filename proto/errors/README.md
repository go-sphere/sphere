# errors proto

This is a proto file that defines errors with status code and message.

### Example

```protobuf
syntax = "proto3";

package dash.v1;

import "sphere/errors/errors.proto";

enum AdminError {
  option (sphere.errors.default_status) = 500;
  ADMIN_ERROR_UNSPECIFIED = 0;
  ADMIN_ERROR_CANNOT_DELETE_SELF = 1001 [(sphere.errors.options) = {
    status: 400
    message: "不能删除当前登录的管理员账号"
  }];
}
```