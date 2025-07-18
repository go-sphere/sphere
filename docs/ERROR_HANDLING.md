# Generating and Handling Errors with Protobuf

Sphere provides a powerful mechanism for generating typed, consistent error-handling code directly from your `.proto` definitions. By defining your errors as enums, you can ensure that error codes, HTTP statuses, and messages are standardized across your application.

This process is handled by `protoc-gen-sphere-errors`, a `protoc` plugin that inspects your `.proto` files and generates Go error-handling code.

## Installation

To install `protoc-gen-sphere-errors`, use the following command:

```bash
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere-errors@latest
```

## Configuration with Buf

To integrate the generator with `buf`, add the plugin to your `buf.gen.yaml` file. This configuration tells `buf` how to execute the plugin and where to place the generated files.

```yaml
version: v2
managed:
  enabled: true
  disable:
    - file_option: go_package_prefix
      module: buf.build/tbxark/errors
  override:
    - file_option: go_package_prefix
      value: github.com/TBXark/sphere/layout/api
plugins:
    # ... other plugins
    - local: protoc-gen-sphere-errors
      out: api # Output directory for generated files
      opt: paths=source_relative
```

## Defining Errors in `.proto`

Errors are defined as `enum` types in your `.proto` files. You can use custom options from `sphere/errors/errors.proto` to attach metadata like HTTP status codes and default messages to each error.

First, import the necessary definitions in your `.proto` file:

```protobuf
import "sphere/errors/errors.proto";
```

Next, define an `enum` for your errors.

### Example: `test.proto`

Here is an example of an error enum from `layout/proto/shared/v1/test.proto`:

```protobuf
syntax = "proto3";

import "sphere/errors/errors.proto";

enum TestError {
  option (sphere.errors.default_status) = 500;
  TEST_ERROR_UNSPECIFIED = 0;
  TEST_ERROR_INVALID_FIELD_TEST1 = 1000 [(sphere.errors.options) = {
    status: 400
    reason: "INVALID_ARGUMENT"
    message: "无效的 field_test1"
  }];
  TEST_ERROR_INVALID_PATH_TEST2 = 1001 [(sphere.errors.options) = {status: 400}];
}
```

### Annotation Reference

*   `(sphere.errors.default_status)`: An enum-level option that sets the default HTTP status code for all values. If an error value does not have a specific status, this one will be used.
*   `(sphere.errors.options)`: A value-level option to customize a specific error.
    *   `status`: The HTTP status code (e.g., `400`, `404`, `500`).
    *   `reason`: A short, stable, machine-readable string identifying the error. This is used as the `Error()` string in the generated Go code.
    *   `message`: A user-facing default error message.

## Using the Generated Code

After running `make gen/api` or `buf generate`, the plugin will create a file named `{proto_name}_errors.pb.go` (e.g., `test_errors.pb.go`). This file contains a Go enum and several helper methods that allow you to use it as a standard Go error.

### Generated Methods

For each `enum TestError`, the following methods are generated:

*   `Error() string`: Returns the error reason, making the type compatible with Go's `error` interface. If `reason` is not set, it returns a string representation of the enum value.
*   `GetCode() int32`: Returns the numeric enum value (e.g., `1000`).
*   `GetStatus() int32`: Returns the configured HTTP status code.
*   `GetMessage() string`: Returns the default error message.
*   `Join(errs ...error) error`: Wraps one or more source errors, returning a `statuserr.Error` that includes the code, status, and message from the enum. This is the recommended way to return an error while preserving the original cause.
*   `JoinWithMessage(msg string, errs ...error) error`: Similar to `Join`, but allows you to provide a custom, dynamic message at runtime.

### Example: Returning an Error in Go

In your service implementation, you can now return one of the generated errors.

```go
package layout

import (
    "fmt"
	sharedv1 "layout/api/shared/v1" // Import the generated package
)

func (s *MyService) SomeBusinessLogic(input string) error {
    if input == "" {
        // Some original error
        originalErr := fmt.Errorf("input cannot be empty")

        // Return the typed error, wrapping the original for context.
		return sharedv1.TestError_TEST_ERROR_INVALID_FIELD_TEST1.Join(originalErr)
    }
    return nil
}
```

When this error is handled by Sphere's server layer, it will automatically be converted into an HTTP response with the correct status code (400) and a JSON body containing the code, reason, and message.
