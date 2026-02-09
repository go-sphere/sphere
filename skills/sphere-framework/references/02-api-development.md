# API Development Guide

> **AI Agent Context**: This is the most important guide for day-to-day development. Use this when users want to add new APIs, create services, or understand the API development workflow. Always refer to this for step-by-step implementation guidance.

## Overview

Sphere follows a **Protocol-First** approach:

```
1. Define Proto → 2. Generate Code → 3. Implement Logic → 4. Done
```

The framework generates HTTP handlers, routes, request binding, validation, and API docs from your `.proto` files, allowing you to focus on business logic.

## Complete Workflow: Adding a New API

### Step 1: Define the Proto Service

Create or edit a `.proto` file in `proto/api/v1/` directory:

**File: `proto/api/v1/product.proto`**

```protobuf
syntax = "proto3";

package api.v1;

import "buf/validate/validate.proto";
import "google/api/annotations.proto";
import "sphere/binding/binding.proto";
import "sphere/errors/errors.proto";

option go_package = "github.com/yourorg/yourproject/api/api/v1;apiv1";

// Product service for managing products
service ProductService {
  // Get a product by ID
  rpc GetProduct(GetProductRequest) returns (Product) {
    option (google.api.http) = {
      get: "/v1/products/{product_id}"
    };
  }

  // List products with search and pagination
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse) {
    option (google.api.http) = {
      get: "/v1/products"
    };
  }

  // Create a new product
  rpc CreateProduct(CreateProductRequest) returns (Product) {
    option (google.api.http) = {
      post: "/v1/products"
      body: "*"
    };
  }

  // Update an existing product
  rpc UpdateProduct(UpdateProductRequest) returns (Product) {
    option (google.api.http) = {
      put: "/v1/products/{product_id}"
      body: "product"
    };
  }

  // Delete a product
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse) {
    option (google.api.http) = {
      delete: "/v1/products/{product_id}"
    };
  }
}

// Messages
message Product {
  int64 id = 1;
  string name = 2;
  string description = 3;
  int64 price = 4;  // in cents
  int64 stock = 5;
  int64 created_at = 6;
  int64 updated_at = 7;
}

message GetProductRequest {
  int64 product_id = 1 [(sphere.binding.location) = BINDING_LOCATION_URI];
}

message ListProductsRequest {
  string search = 1 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int32 page = 2 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int32 page_size = 3 [
    (sphere.binding.location) = BINDING_LOCATION_QUERY,
    (buf.validate.field).int32 = {lte: 100}
  ];
}

message ListProductsResponse {
  repeated Product products = 1;
  int32 total = 2;
}

message CreateProductRequest {
  string name = 1 [(buf.validate.field).string.min_len = 1];
  string description = 2;
  int64 price = 3 [(buf.validate.field).int64.gte = 0];
  int64 stock = 4 [(buf.validate.field).int64.gte = 0];
}

message UpdateProductRequest {
  int64 product_id = 1 [(sphere.binding.location) = BINDING_LOCATION_URI];
  Product product = 2;
}

message DeleteProductRequest {
  int64 product_id = 1 [(sphere.binding.location) = BINDING_LOCATION_URI];
}

message DeleteProductResponse {
  bool success = 1;
}

// Error definitions
enum ProductError {
  option (sphere.errors.default_status) = 500;
  PRODUCT_ERROR_UNSPECIFIED = 0;
  PRODUCT_ERROR_NOT_FOUND = 10001 [(sphere.errors.options) = {
    status: 404
    reason: "PRODUCT_NOT_FOUND"
    message: "Product not found"
  }];
  PRODUCT_ERROR_INVALID_PRICE = 10002 [(sphere.errors.options) = {
    status: 400
    reason: "INVALID_PRICE"
    message: "Product price must be non-negative"
  }];
  PRODUCT_ERROR_OUT_OF_STOCK = 10003 [(sphere.errors.options) = {
    status: 400
    reason: "OUT_OF_STOCK"
    message: "Product is out of stock"
  }];
}
```

**Key Points:**

1. **HTTP Annotations**: Use `google.api.http` to define HTTP method and path
2. **Binding Locations**: Use `sphere.binding.location` to specify where parameters come from:
   - `BINDING_LOCATION_URI`: Path parameters (e.g., `/products/{id}`)
   - `BINDING_LOCATION_QUERY`: Query parameters (e.g., `?search=apple`)
   - `BINDING_LOCATION_HEADER`: HTTP headers
   - Default (no annotation): JSON body
3. **Validation**: Use `buf.validate` for request validation
4. **Errors**: Define errors as enums with HTTP status codes

### Step 2: Generate Code

```bash
# Generate protobuf code, routes, errors, and Swagger docs
make gen/proto

# This runs:
# - buf generate (generates Go code from proto)
# - protoc-gen-sphere (generates HTTP handlers)
# - protoc-gen-sphere-binding (adds struct tags)
# - protoc-gen-sphere-errors (generates error code)
# - swag init (generates Swagger docs)
```

**Expected output:**
```
✓ Generated: api/api/v1/product.pb.go           # Proto messages
✓ Generated: api/api/v1/product.sphere.go       # HTTP handlers and routes
✓ Generated: api/api/v1/product_errors.go       # Error definitions
✓ Swagger docs updated
```

**IMPORTANT**: Never edit these generated files manually!

### Step 3: Implement the Service

Create a service implementation file:

**File: `internal/service/api/product.go`**

```go
package api

import (
    "context"
    "fmt"

    apiv1 "github.com/yourorg/yourproject/api/api/v1"
    "github.com/yourorg/yourproject/internal/pkg/database/ent"
    "github.com/yourorg/yourproject/internal/pkg/database/ent/product"
)

func (s *Service) GetProduct(ctx context.Context, req *apiv1.GetProductRequest) (*apiv1.Product, error) {
    // Query from database
    p, err := s.db.Product.
        Query().
        Where(product.ID(req.ProductId)).
        Only(ctx)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, apiv1.ProductError_PRODUCT_ERROR_NOT_FOUND
        }
        return nil, err
    }

    // Convert ent entity to protobuf
    return s.render.Product(p), nil
}

func (s *Service) ListProducts(ctx context.Context, req *apiv1.ListProductsRequest) (*apiv1.ListProductsResponse, error) {
    query := s.db.Product.Query()

    // Apply search filter
    if req.Search != "" {
        query = query.Where(product.NameContains(req.Search))
    }

    // Count total
    total, err := query.Count(ctx)
    if err != nil {
        return nil, err
    }

    // Apply pagination
    page := req.Page
    if page < 1 {
        page = 1
    }
    pageSize := req.PageSize
    if pageSize < 1 {
        pageSize = 20
    }

    products, err := query.
        Offset(int((page - 1) * pageSize)).
        Limit(int(pageSize)).
        All(ctx)
    if err != nil {
        return nil, err
    }

    return &apiv1.ListProductsResponse{
        Products: s.render.Products(products),
        Total:    int32(total),
    }, nil
}

func (s *Service) CreateProduct(ctx context.Context, req *apiv1.CreateProductRequest) (*apiv1.Product, error) {
    // Validate price
    if req.Price < 0 {
        return nil, apiv1.ProductError_PRODUCT_ERROR_INVALID_PRICE
    }

    // Create product
    p, err := s.db.Product.
        Create().
        SetName(req.Name).
        SetDescription(req.Description).
        SetPrice(req.Price).
        SetStock(req.Stock).
        Save(ctx)
    if err != nil {
        return nil, err
    }

    return s.render.Product(p), nil
}

func (s *Service) UpdateProduct(ctx context.Context, req *apiv1.UpdateProductRequest) (*apiv1.Product, error) {
    // Check if product exists
    exists, err := s.db.Product.Query().Where(product.ID(req.ProductId)).Exist(ctx)
    if err != nil {
        return nil, err
    }
    if !exists {
        return nil, apiv1.ProductError_PRODUCT_ERROR_NOT_FOUND
    }

    // Update product
    p, err := s.db.Product.
        UpdateOneID(req.ProductId).
        SetName(req.Product.Name).
        SetDescription(req.Product.Description).
        SetPrice(req.Product.Price).
        SetStock(req.Product.Stock).
        Save(ctx)
    if err != nil {
        return nil, err
    }

    return s.render.Product(p), nil
}

func (s *Service) DeleteProduct(ctx context.Context, req *apiv1.DeleteProductRequest) (*apiv1.DeleteProductResponse, error) {
    // Delete product
    err := s.db.Product.DeleteOneID(req.ProductId).Exec(ctx)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, apiv1.ProductError_PRODUCT_ERROR_NOT_FOUND
        }
        return nil, err
    }

    return &apiv1.DeleteProductResponse{
        Success: true,
    }, nil
}
```

### Step 4: Register the Service

Edit the web server file to register your service:

**File: `internal/server/api/web.go`**

```go
func (w *Web) Start(ctx context.Context) error {
    // ... existing middleware setup ...

    // Register ProductService
    apiv1.RegisterProductServiceHTTPServer(needAuthRoute, w.service)

    // ... rest of the code ...
}
```

### Step 5: (If New Service Struct) Update Dependency Injection

If you created a new Service struct or added dependencies, regenerate Wire:

**Edit: `internal/service/api/wire.go`** (if needed)

```go
var ProviderSet = wire.NewSet(
    NewService,
    // Add new providers here if needed
)
```

Then run:

```bash
make gen/wire
```

### Step 6: Test the API

```bash
# Start the server
make run

# Test in another terminal:

# Get product
curl http://localhost:8899/v1/products/1

# List products
curl http://localhost:8899/v1/products?search=apple&page=1&page_size=10

# Create product
curl -X POST http://localhost:8899/v1/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "iPhone 15",
    "description": "Latest iPhone model",
    "price": 99900,
    "stock": 100
  }'

# Update product
curl -X PUT http://localhost:8899/v1/products/1 \
  -H "Content-Type: application/json" \
  -d '{
    "product": {
      "name": "iPhone 15 Pro",
      "description": "Pro model",
      "price": 119900,
      "stock": 50
    }
  }'

# Delete product
curl -X DELETE http://localhost:8899/v1/products/1

# View API docs
open http://localhost:8899/swagger/index.html
```

## Using sphere-cli for Code Generation (Optional Helper)

While sphere-cli **cannot** automatically create files or generate complete working code, it **can** generate reference code snippets that you manually copy into your files:

### Generate Proto File Template

```bash
sphere-cli service proto --name Product --package api.v1

# Output will be printed to stdout, for example:
# syntax = "proto3";
# package api.v1;
# ...
# (You manually copy this into proto/api/v1/product.proto)
```

### Generate Go Service Implementation Template

```bash
sphere-cli service golang --name Product --package api.v1 --mod github.com/yourorg/yourproject

# Output will be printed to stdout
# (You manually copy this into internal/service/api/product.go)
```

**Note**: These commands only print template code to your terminal. You need to:
1. Create the corresponding `.proto` or `.go` file yourself
2. Copy and paste the generated code
3. Modify the code to fit your actual requirements

## Request Binding Patterns

### Pattern 1: URI + Query + Body (Mixed)

```protobuf
rpc UpdateUser(UpdateUserRequest) returns (User) {
  option (google.api.http) = {
    put: "/v1/users/{user_id}"
    body: "user"
  };
}

message UpdateUserRequest {
  int64 user_id = 1 [(sphere.binding.location) = BINDING_LOCATION_URI];
  string reason = 2 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  User user = 3;  // Goes to JSON body
}
```

Generated handler automatically binds:
- `user_id` from path (`/v1/users/123`)
- `reason` from query (`?reason=update_profile`)
- `user` from JSON body

### Pattern 2: Query-Only (Search/Filter)

```protobuf
rpc SearchUsers(SearchUsersRequest) returns (SearchUsersResponse) {
  option (google.api.http) = {get: "/v1/users/search"};
}

message SearchUsersRequest {
  string query = 1 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  repeated string fields = 2 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int32 limit = 3 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
}
```

### Pattern 3: Header Authentication

```protobuf
message AuthenticatedRequest {
  string authorization = 1 [(sphere.binding.location) = BINDING_LOCATION_HEADER];
  string x_request_id = 2 [(sphere.binding.location) = BINDING_LOCATION_HEADER];
}
```

## Error Handling Best Practices

### Define Domain-Specific Errors

```protobuf
enum OrderError {
  option (sphere.errors.default_status) = 500;
  ORDER_ERROR_UNSPECIFIED = 0;

  // 4xx Client Errors
  ORDER_ERROR_INVALID_QUANTITY = 20001 [(sphere.errors.options) = {
    status: 400
    reason: "INVALID_QUANTITY"
    message: "Order quantity must be positive"
  }];
  ORDER_ERROR_PRODUCT_NOT_FOUND = 20002 [(sphere.errors.options) = {
    status: 404
    message: "Product not found in order"
  }];
  ORDER_ERROR_INSUFFICIENT_STOCK = 20003 [(sphere.errors.options) = {
    status: 409
    reason: "INSUFFICIENT_STOCK"
    message: "Not enough product stock available"
  }];

  // 5xx Server Errors
  ORDER_ERROR_PAYMENT_FAILED = 20100 [(sphere.errors.options) = {
    status: 502
    reason: "PAYMENT_SERVICE_ERROR"
    message: "Payment service is unavailable"
  }];
}
```

### Use Errors in Service Logic

```go
func (s *Service) CreateOrder(ctx context.Context, req *apiv1.CreateOrderRequest) (*apiv1.Order, error) {
    // Validate quantity
    if req.Quantity <= 0 {
        return nil, apiv1.OrderError_ORDER_ERROR_INVALID_QUANTITY
    }

    // Check product exists
    product, err := s.db.Product.Get(ctx, req.ProductId)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, apiv1.OrderError_ORDER_ERROR_PRODUCT_NOT_FOUND
        }
        return nil, err
    }

    // Check stock
    if product.Stock < req.Quantity {
        return nil, apiv1.OrderError_ORDER_ERROR_INSUFFICIENT_STOCK
    }

    // Process payment (external service)
    if err := s.paymentService.Charge(ctx, req.Amount); err != nil {
        // Wrap error with context
        return nil, apiv1.OrderError_ORDER_ERROR_PAYMENT_FAILED.Join(err)
    }

    // Create order...
    return order, nil
}
```

## Validation with buf.validate

Add validation rules in proto:

```protobuf
message CreateUserRequest {
  string email = 1 [(buf.validate.field).string = {
    email: true
    min_len: 5
    max_len: 100
  }];

  string password = 2 [(buf.validate.field).string = {
    min_len: 8
    max_len: 72
  }];

  int32 age = 3 [(buf.validate.field).int32 = {
    gte: 18
    lte: 120
  }];

  string phone = 4 [(buf.validate.field).string.pattern = "^\\+?[1-9]\\d{1,14}$"];

  repeated string tags = 5 [(buf.validate.field).repeated = {
    min_items: 1
    max_items: 10
  }];
}
```

Validation happens automatically in generated handlers before reaching your service logic.

## Response Customization

### Custom Response Body Field

```protobuf
rpc BatchUpdate(BatchUpdateRequest) returns (BatchUpdateResponse) {
  option (google.api.http) = {
    post: "/v1/batch/update"
    body: "*"
    response_body: "results"  // Only return the "results" field
  };
}

message BatchUpdateResponse {
  repeated UpdateResult results = 1;
  map<string, string> metadata = 2;  // Won't be in HTTP response
}
```

### Streaming Not Supported

Sphere focuses on REST APIs. For streaming, consider using raw gRPC or WebSocket.

## Common Patterns

### Pagination

```protobuf
message ListRequest {
  int32 page = 1 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int32 page_size = 2 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  string cursor = 3 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
}

message ListResponse {
  repeated Item items = 1;
  int32 total = 2;
  string next_cursor = 3;
}
```

### Filtering

```protobuf
message FilterRequest {
  string status = 1 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  repeated string tags = 2 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int64 created_after = 3 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int64 created_before = 4 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
}
```

### Batch Operations

```protobuf
message BatchDeleteRequest {
  repeated int64 ids = 1;
}

message BatchDeleteResponse {
  int32 deleted_count = 1;
  repeated int64 failed_ids = 2;
}
```

## Troubleshooting

### Issue: Generated code doesn't import new service

**Cause**: Service not registered in web server

**Solution**: Add `RegisterXXXServiceHTTPServer(route, service)` in `internal/server/api/web.go`

### Issue: "undefined: NewService" after adding dependencies

**Cause**: Wire providers not updated

**Solution**:
1. Add new providers to `internal/*/wire.go`
2. Run `make gen/wire`

### Issue: Request binding not working

**Cause**: Wrong binding location or missing `sphere.binding.location` annotation

**Solution**: Check if binding tags are correctly generated in `.pb.go` file after `make gen/proto`

### Issue: Validation not working

**Cause**: Missing `protovalidate.Validate(&in)` in generated handler

**Solution**: Ensure `buf.validate` is imported in proto and `make gen/proto` was run

## AI Agent Checklist

When implementing a new API:

- [ ] Proto file created/edited in `proto/api/v1/`
- [ ] HTTP annotations (`google.api.http`) added
- [ ] Binding locations specified for path/query params
- [ ] Validation rules added (`buf.validate`)
- [ ] Error enum defined with HTTP status codes
- [ ] `make gen/proto` executed successfully
- [ ] Service methods implemented in `internal/service/api/`
- [ ] Service registered in `internal/server/api/web.go`
- [ ] `make gen/wire` run (if new dependencies added)
- [ ] `make run` starts without errors
- [ ] API tested with curl or Swagger UI
- [ ] Errors return correct HTTP status codes
