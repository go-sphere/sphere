# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sphere is a Go-based backend scaffolding framework for building monolithic applications with a focus on simplicity, maintainability, and scalability. It uses ent for database schema management and proto for API definitions, with powerful code generation tools.

## Architecture Overview

### Core Components
- **Web Framework**: Gin-gonic/gin for HTTP handling
- **ORM**: ent for schema management and database operations  
- **Dependency Injection**: Google wire for dependency management
- **API Definition**: Protobuf with gRPC-Gateway for HTTP transcoding
- **Documentation**: Swagger/OpenAPI generation via swaggo

### Project Structure
```
sphere/                    # Core framework library
cmd/                       # CLI tools and code generators
  protoc-gen-sphere/       # Protobuf HTTP server generator
  protoc-gen-route/        # Protobuf routing generator  
  protoc-gen-sphere-binding/ # Struct binding tag generator
  protoc-gen-sphere-errors/ # Error handling generator
  sphere-cli/              # Main CLI for project management
layout/                    # Standard project template
  cmd/app/                 # Application entry point
  internal/                # Private application code
    biz/                   # Business logic layer
    config/               # Configuration management
    pkg/                  # Shared internal packages
    server/               # Server implementations (API, Dashboard, Bot)
    service/              # Service layer with API implementations
  proto/                  # Protobuf definitions
  swagger/                # Generated OpenAPI documentation
```

## Development Commands

### Core Framework Commands
```bash
make install    # Install all CLI tools and dependencies
make fmt        # Format code and run linting across all modules
make test       # Run tests for all modules
make upgrade    # Upgrade all dependencies
make nilaway    # Run nilaway static analysis
```

### Project Template Commands (layout/ directory)
```bash
cd layout
make init              # Initialize all dependencies and generate code
make gen/all           # Generate all code (clean + docs + wire)
make gen/db            # Generate ent database code from schemas
make gen/proto         # Generate protobuf and HTTP bindings
make gen/docs          # Generate Swagger documentation
make gen/wire          # Generate wire dependency injection code
make gen/dts           # Generate TypeScript API clients

make build             # Build binary for current architecture  
make build/all         # Build for all platforms (linux/amd64, linux/arm64)
make run               # Run the application locally
make lint              # Run golangci-lint and buf linting
make fmt               # Format code and fix issues
```

### CLI Tools Installation and Usage

#### Sphere CLI (Project Creation)
```bash
# Install sphere-cli
go install github.com/TBXark/sphere/cmd/sphere-cli@latest

# Create new project
sphere-cli create --name myproject --mod github.com/user/myproject

# Generate service code
sphere-cli service golang --name MyService
sphere-cli service proto --name MyService
```

#### Code Generators (Install individually)
```bash
# Install protoc plugins
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere@latest
go install github.com/TBXark/sphere/cmd/protoc-gen-route@latest  
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere-errors@latest
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere-binding@latest
```

## Key Development Workflows

### 1. Creating a New Service
1. Define database schema in `layout/internal/pkg/database/ent/schema/`
2. Run `make gen/db` to generate ent code
3. Define API in protobuf files in `layout/proto/`
4. Run `make gen/proto` to generate HTTP bindings
5. Implement service logic in `layout/internal/service/`
6. Run `make gen/all` to regenerate all code

### 2. Database Schema Changes
1. Modify ent schema files
2. Run `make gen/db` (also triggers proto generation)
3. Run `make gen/wire` to update dependency injection

### 3. API Definition Changes  
1. Edit protobuf files with gRPC-Gateway annotations
2. Run `make gen/proto` to regenerate bindings
3. Run `make gen/docs` to update Swagger documentation

### 4. Testing Changes
```bash
# Test entire project
cd layout && make gen/all && make build

# Test specific components
make test                    # Core framework tests
cd layout && go test ./...   # Project template tests
```

## Module Structure

The project contains multiple Go modules:
- **Root module**: github.com/TBXark/sphere (core framework)
- **Layout module**: github.com/TBXark/sphere/layout (project template)
- **Proto submodules**: binding, errors, options (shared protobuf definitions)
- **CLI tools**: Individual modules for each protoc plugin

## Configuration Management

Applications use structured configuration loaded via confstore:
- Configuration defined in `layout/internal/config/config.go`
- Supports JSON/YAML configuration files
- Environment variable overrides available
- Separate configs for API, Dashboard, Bot, Database, Storage, etc.

## Code Generation Pipeline

1. **Database First**: ent schemas → ent client + protobuf messages
2. **API Definition**: protobuf files → HTTP bindings + Swagger docs  
3. **Dependency Injection**: wire annotations → generated constructors
4. **TypeScript Client**: Swagger docs → TypeScript API bindings

## Testing Approach

- Unit tests for core framework components
- Integration tests for generated code in layout template
- CLI command tests in sphere-cli
- Automated testing via `make test` across all modules

## Build and Deployment

- Single binary deployment via `make build`
- Docker support via `make build/docker`
- Multi-architecture builds via `make build/multi-docker`
- Environment-specific configuration management