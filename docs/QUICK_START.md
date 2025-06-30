# Quick Start Guide

This guide provides a step-by-step walkthrough for creating, building, and running a new application using the Sphere framework.

### Prerequisites

Before you begin, ensure you have the following installed:
*   Go (version 1.24 or later)
*   Docker and Docker Compose
*   Node.js and npm (for TypeScript client generation)

### 1. Create a New Project

The first step is to install the `sphere-cli` and use it to bootstrap your new application. The CLI will create a new project directory with the recommended layout.

```bash
# Install the command-line tool
go install github.com/TBXark/sphere/cmd/sphere-cli@latest

# Create a new project using the template
# Replace 'myproject' with your project name and update the Go module path
sphere-cli create --name myproject --mod github.com/TBXark/myproject
```

This command generates a new project with a clean structure. `sphere` is designed to be flexible, so feel free to modify the directory layout to suit your project's needs.

### 2. Define Database Schema with Ent

Sphere uses `ent` to manage the database schema. Define your database entities (tables) in the `/internal/pkg/database/ent/schema` directory.

For a detailed guide on writing `ent` schemas, refer to the [official ent documentation](https://entgo.io/docs/getting-started).

After defining your schemas, run the following `make` command:

```bash
# This command generates:
# - Ent client and schema code in /internal/pkg/database/ent
# - Corresponding Protobuf message definitions in /proto/entpb
make gen/db
```

### 3. Define API Interfaces with Protobuf

Your service's API endpoints are defined in `.proto` files located in the `/proto` directory. Sphere utilizes [gRPC-Gateway](https://grpc-ecosystem.github.io/grpc-gateway/) style annotations to define how gRPC services map to HTTP/JSON endpoints.

For more details on writing `.proto` files and using transcoding, see:
*   [Protobuf Language Guide (proto3)](https://developers.google.com/protocol-buffers/docs/proto3)
*   [gRPC Transcoding](https://cloud.google.com/endpoints/docs/grpc/transcoding)
*   [API Definition Rules](API_DEFINITIONS.md) for Sphere-specific conventions.

Once your API is defined, generate the server code, client stubs, and Swagger/OpenAPI documentation:

```bash
# This command generates:
# - Go server stubs in the /api directory
# - Swagger/OpenAPI v2 specifications in the /swagger directory
make gen/docs
```

You can also generate a TypeScript client for your frontend application:
```bash
make gen/dts
```

### 4. Implement Business Logic

With the data models and API interfaces generated, you can now implement your application's business logic. The recommended structure is:

*   `/internal/biz`: Contains the core business logic and use cases. This layer is independent of the transport layer (gRPC/HTTP).
*   `/internal/service`: Implements the gRPC service interfaces defined in your `.proto` files. This layer acts as an adapter, translating incoming requests into calls to the `biz` layer.

### 5. Assemble and Run the Server

The application's main entry point is in the `cmd/app` directory. Sphere uses [Google Wire](https://github.com/google/wire) for dependency injection to wire together all the components (database, services, servers).

First, generate the dependency injection code:
```bash
make gen/wire
```

Finally, start the application server:
```bash
make run
```

Your server should now be running and accessible. You can also run the Swagger UI to explore and test your API:
```bash
make run/swag
```
