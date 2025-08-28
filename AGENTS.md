# AGENT.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sphere is a Go-based backend scaffolding framework for building monolithic applications with a focus on simplicity, maintainability, and scalability. It uses ent for database schema management and proto for API definitions, with powerful code generation tools.

## Architecture Overview

### Core Architecture
- **Database First**: ent schemas → protobuf messages → HTTP APIs → TypeScript clients
- **Code Generation**: Heavy automation via protoc plugins and CLI tools
- **Layered Design**: Schema → DAO → Service → API → Client
- **Single Binary**: Everything compiles to one deployable binary

### Key Components
- **Web Framework**: Gin-gonic/gin for HTTP handling
- **ORM**: ent for schema management and database operations
- **Dependency Injection**: Google wire for dependency management
- **API Definition**: Protobuf with gRPC-Gateway for HTTP transcoding
- **Documentation**: Swagger/OpenAPI generation via swaggo

### Project Structure
```
sphere/                    # Core framework library
├── cmd/                   # CLI tools and code generators
│   ├── sphere-cli/       # Main CLI for project management
│   ├── protoc-gen-sphere/ # HTTP server generator
│   ├── protoc-gen-route/  # Routing generator
│   ├── protoc-gen-sphere-binding/ # Struct tag generator
│   └── protoc-gen-sphere-errors/ # Error handling generator
├── layout/               # Standard project template
│   ├── cmd/app/          # Application entry point
│   ├── internal/         # Private application code
│   │   ├── biz/          # Business logic layer
│   │   ├── config/       # Configuration management
│   │   ├── pkg/          # Shared internal packages
│   │   ├── server/       # Server implementations
│   │   └── service/      # Service layer with API implementations
│   └── proto/            # Protobuf definitions
└── proto/                # Shared protobuf definitions
    ├── binding/          # Field binding options
    ├── errors/           # Error handling definitions
    └── options/          # API options
```

## Essential Commands

### Core Framework (root directory)
```bash
make install    # Install all CLI tools and dependencies
make fmt        # Format code and run linting across all modules
make test       # Run tests for all modules
make upgrade    # Upgrade all dependencies
make nilaway    # Run nilaway static analysis
```

### Project Development (layout/ directory)
```bash
# Setup and generation
make init              # Initialize all dependencies and generate code
make gen/all           # Generate all code (clean + docs + wire)
make gen/db            # Generate ent database code from schemas
make gen/proto         # Generate protobuf and HTTP bindings
make gen/docs          # Generate Swagger documentation
make gen/wire          # Generate wire dependency injection code
make gen/dts           # Generate TypeScript API clients

# Build and run
make build             # Build binary for current architecture
make build/all         # Build for all platforms
make run               # Run the application locally
make lint              # Run golangci-lint and buf linting
make fmt               # Format code and fix issues
```

### CLI Tools Installation
```bash
# Install sphere-cli
go install github.com/go-sphere/sphere-cli@latest

# Install protoc plugins
go install github.com/go-sphere/protoc-gen-sphere@latest
go install github.com/go-sphere/protoc-gen-route@latest
go install github.com/go-sphere/protoc-gen-sphere-errors@latest
go install github.com/go-sphere/protoc-gen-sphere-binding@latest
```

## Development Workflow

### 1. Creating a New Service
1. **Define schema**: Edit `layout/internal/pkg/database/ent/schema/*.go`
2. **Generate database code**: `make gen/db`
3. **Define API**: Edit `layout/proto/**/*.proto`
4. **Generate bindings**: `make gen/proto`
5. **Implement service**: Edit `layout/internal/service/**/*.go`
6. **Regenerate everything**: `make gen/all`

### 2. Code Generation Pipeline
```
Ent Schema → Protobuf Messages → HTTP Bindings → Swagger Docs → TypeScript Client
    ↓            ↓               ↓            ↓              ↓
make gen/db → make gen/proto → make gen/docs → make gen/dts
```

### 3. Testing Single Components
```bash
# Test specific module
cd layout && go test ./internal/service/api/...

# Test with race detection
cd layout && go test -race ./...

# Test specific function
cd layout && go test -run TestUserService_GetUser ./internal/service/api/...
```

## Key Files to Know

### Configuration
- `layout/config.json` - Main application configuration
- `layout/internal/config/config.go` - Configuration structure
- `layout/buf.yaml` - Protobuf configuration

### Entry Points
- `layout/cmd/app/main.go` - Application entry point
- `layout/cmd/app/wire.go` - Dependency injection setup

### Core Components
- `layout/internal/pkg/database/ent/` - Generated database code
- `layout/internal/service/` - Service implementations
- `layout/internal/server/` - HTTP server configurations
- `layout/proto/` - API definitions

## Common Patterns

### Database Operations
- Use ent client: `s.db.User.Get(ctx, id)`
- Transactions: `s.db.Tx(ctx, func(tx *ent.Tx) error { ... })`
- Pagination: Use ent's built-in pagination

### API Design
- Use protobuf for all API definitions
- Follow RESTful conventions via gRPC-Gateway
- Shared messages in `proto/shared/v1/`
- Error handling via `proto/errors/`

### Service Implementation
- Services implement protobuf interfaces
- Use render functions for entity → protobuf conversion
- Direct DAO access from services for CRUD operations
- Complex business logic in `internal/biz/`

## Module Structure

The project contains multiple Go modules:
- **Root**: github.com/go-sphere/sphere (core framework)
- **Layout**: github.com/go-sphere/sphere-layout (project template)
- **Proto submodules**: binding, errors, options (shared protobuf definitions)
- **CLI tools**: Individual modules for each protoc plugin

## Environment Setup

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- Node.js & npm (for TypeScript client generation)
- protoc compiler

### Quick Setup
```bash
# Install sphere-cli
go install github.com/go-sphere/sphere-cli@latest

# Create new project
sphere-cli create --name myproject --mod github.com/user/myproject

# Initialize
cd myproject
make init
```