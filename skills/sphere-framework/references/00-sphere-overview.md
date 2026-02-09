# Sphere Framework Overview

> **AI Agent Context**: This document provides a high-level overview of the Sphere framework. Use this to understand the framework's philosophy, architecture, and key components before diving into specific tasks.

## What is Sphere?

Sphere is a **Protocol-First** Go backend development framework that follows these principles:

1. **Define First, Generate Later**: Define APIs in Protocol Buffers, then auto-generate HTTP servers, routes, error handling, API docs, and TypeScript clients
2. **Modular Monolith**: Start with a single codebase that can seamlessly scale to microservices
3. **Production-Ready**: Includes auth, caching, storage, logging, and observability out of the box
4. **Code Generation Driven**: Minimize boilerplate through extensive code generation

## Core Philosophy

```
┌─────────────────┐
│  Proto Files    │  Define your API contract
│  (.proto)       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Code Gen       │  Generate HTTP server, routes, errors, docs
│  (make gen)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Implement      │  Write business logic only
│  (service/)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Deploy         │  Single binary, Docker, or K8s
└─────────────────┘
```

## Key Components

### 1. CLI Tool (sphere-cli)

```bash
# Create new project from template
sphere-cli create --name my-project --module github.com/yourorg/my-project

# List available templates
sphere-cli create list

# Generate proto code template (prints to stdout, manual copy needed)
sphere-cli service proto --name User --package api.v1

# Generate Go service template (prints to stdout, manual copy needed)
sphere-cli service golang --name User --package api.v1 --mod github.com/yourorg/my-project

# Rename Go module across project
sphere-cli rename --old old-module --new new-module --target .
```

**Note**: `sphere-cli service` commands only print template code to stdout. You must manually create files and copy the generated code.

### 2. Protoc Code Generators

- **protoc-gen-sphere**: Generate HTTP handlers and route registration
- **protoc-gen-sphere-binding**: Request binding (path params, query, body)
- **protoc-gen-sphere-errors**: Type-safe error definitions with HTTP status
- **protoc-gen-route**: Custom TypeScript route definitions

### 3. Core Packages

| Package | Purpose | Example Use |
|---------|---------|-------------|
| `httpx` | HTTP framework adapter | Support Gin/Fiber/Echo/Hertz |
| `cache` | Caching layer | Redis/Memory/BadgerDB |
| `storage` | File storage | S3/Local/Qiniu |
| `auth` | Authentication | JWT + RBAC + ACL |
| `confstore` | Configuration | JSON/YAML/Remote HTTP |
| `log` | Structured logging | Zap-based logger |

### 4. ORM Integration (Ent)

- **Ent ORM**: Type-safe, code-generated ORM
- **entc-extensions**: Generate Protobuf definitions from Ent schemas
- Bidirectional conversion: `ent.User` ↔ `protobuf.User`

## Project Structure (sphere-layout template)

```
my-project/
├── cmd/app/              # Application entry point
│   ├── main.go           # Starts the app
│   ├── builder.go        # Wire all components together
│   └── wire.go           # Dependency injection config
│
├── proto/                # Protocol Buffer source
│   ├── api/v1/*.proto    # API definitions
│   └── buf.yaml          # Buf configuration
│
├── api/                  # Generated Go code from proto
│   └── api/v1/*.pb.go    # Auto-generated, don't edit
│
├── internal/
│   ├── config/           # Configuration structs
│   ├── server/           # HTTP servers (api, dash, file)
│   │   ├── api/          # API server (public REST API)
│   │   ├── dash/         # Dashboard server (admin backend)
│   │   └── fileserver/   # File serving
│   │
│   ├── service/          # Business logic implementation
│   │   ├── api/          # Implements API proto services
│   │   └── dash/         # Implements Dashboard proto services
│   │
│   ├── biz/              # Lifecycle tasks (init, cleanup)
│   └── pkg/
│       ├── database/
│       │   ├── schema/   # Ent schemas (YOUR models)
│       │   └── ent/      # Generated Ent code
│       ├── dao/          # Data Access Objects
│       └── auth/         # Auth logic
│
├── devops/               # Docker and deployment
├── swagger/              # Auto-generated API docs
└── Makefile              # Development commands
```

## Development Workflow

### Typical Flow for Adding a New Feature

```bash
# 1. Define data model (if needed)
#    Edit: internal/pkg/database/schema/user.go
make gen/db                    # Generate Ent code

# 2. Define API
#    Edit: proto/api/v1/user.proto
make gen/proto                 # Generate HTTP server, routes, errors
make gen/docs                  # Generate Swagger docs

# 3. Implement service logic
#    Edit: internal/service/api/user.go
#    Implement the protobuf service interface

# 4. Wire dependencies (if new services added)
make gen/wire                  # Generate dependency injection

# 5. Run and test
make run                       # Start the server
```

## Key Concepts

### 1. Protocol-First Development

Instead of writing HTTP handlers manually:

```go
// ❌ Traditional way
func GetUser(c *gin.Context) {
    userID := c.Param("id")
    // Parse, validate, handle...
}
```

You define in Protobuf:

```protobuf
// ✅ Sphere way
message GetUserRequest {
  int64 user_id = 1 [(sphere.binding.location) = BINDING_LOCATION_URI];
}

service UserService {
  rpc GetUser(GetUserRequest) returns (User) {
    option (google.api.http) = {get: "/v1/users/{user_id}"};
  }
}
```

Sphere generates:
- HTTP route: `GET /v1/users/:user_id`
- Request binding code
- Response serialization
- OpenAPI documentation

### 2. Dependency Injection with Wire

All components are wired together using Google Wire:

```go
// cmd/app/builder.go
type Application struct {
    config  *config.Config
    servers []app.Server    // All HTTP servers
    tasks   []app.Bootable  // Lifecycle tasks
}

// Wire automatically generates initialization code
```

### 3. Multi-Server Architecture

A single application can run multiple servers:

- **API Server** (`:8899`): Public REST API
- **Dashboard Server** (`:8800`): Admin backend with RBAC
- **File Server** (`:9900`): Static file serving
- **Bot Server**: Telegram/Discord bot (optional)

### 4. Lifecycle Management

Services implement the `Bootable` interface:

```go
type Bootable interface {
    Boot(ctx context.Context) error    // Init phase
    Start(ctx context.Context) error   // Start phase
    Stop(ctx context.Context) error    // Graceful shutdown
}
```

The framework manages startup order and graceful shutdown.

## Common Commands

```bash
# Code generation
make gen/all        # Generate everything (ent, proto, docs, wire)
make gen/proto      # Generate from .proto files
make gen/db         # Generate Ent ORM code
make gen/wire       # Generate dependency injection
make gen/docs       # Generate Swagger documentation

# Development
make run            # Run the application
make build          # Build binary
make test           # Run tests

# Docker
make build/docker   # Build Docker image
make up             # Start with docker-compose
```

## Key File Paths Reference

| Purpose | File Path | Description |
|---------|-----------|-------------|
| App entry | `cmd/app/main.go` | Application entry point |
| Configuration | `internal/config/config.go` | Config struct definition |
| Wire config | `cmd/app/wire.go` | Dependency injection |
| API proto | `proto/api/v1/*.proto` | API definitions |
| Ent schemas | `internal/pkg/database/schema/*.go` | Data models |
| Service impl | `internal/service/api/*.go` | Business logic |
| Servers | `internal/server/*/web.go` | HTTP server setup |
| Config file | `config.json` or `config.yaml` | Runtime configuration |

## When to Use Sphere

### ✅ Good Fit

- Building REST APIs with clear contracts
- Projects requiring API documentation
- Need frontend TypeScript integration
- Want strong typing and code generation
- Building modular monoliths that may scale to microservices

### ❌ Not Ideal

- GraphQL-primary projects (though you can add it)
- Websocket-heavy applications (requires custom implementation)
- Pure gRPC services (though Sphere uses gRPC internally)
- Projects preferring manual control over code generation

## Next Steps

- **Creating a new project**: See `01-getting-started.md`
- **Defining APIs**: See `02-api-development.md`
- **Database setup**: See `03-database-orm.md`
- **Authentication**: See `04-auth-permissions.md`
- **Quick reference**: See `99-quick-reference.md`

## Important Notes for AI Agents

1. **Always run `make gen/proto` after editing .proto files**
2. **Never edit files in `api/` or `internal/pkg/database/ent/` directories** - they are auto-generated
3. **Use Wire for dependency injection** - manual initialization will break the pattern
4. **Follow the project structure** - don't create new top-level directories
5. **Check the Makefile** - it contains all standard development commands
