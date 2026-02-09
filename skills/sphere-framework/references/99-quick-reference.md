# Quick Reference and Troubleshooting

> **AI Agent Context**: Use this as a quick lookup for common commands, file paths, and solutions to frequent issues. This is organized for rapid problem-solving.

## Quick Command Reference

### Code Generation

```bash
# Generate all code
make gen/all

# Individual generators
make gen/proto        # Protobuf → Go code
make gen/db           # Ent schema → ORM code
make gen/wire         # Dependency injection
make gen/docs         # Swagger documentation

# Manual commands (if Makefile unavailable)
buf generate                                    # Generate proto
go run ./cmd/tools/ent generate ./schema        # Generate Ent
wire ./cmd/app                                  # Generate Wire
swag init -g cmd/app/main.go                    # Generate Swagger
```

### Development

```bash
# Run application
make run

# Build binary
make build

# Run tests
make test

# Clean generated files
make clean

# Format code
make fmt

# Lint code
make lint
```

### Docker

```bash
# Build Docker image
make build/docker

# Run with docker-compose
make up

# Stop containers
make down

# View logs
docker-compose logs -f
```

## Critical File Paths

### Source Files (Edit These)

| Purpose | Path | Description |
|---------|------|-------------|
| **Proto definitions** | `proto/api/v1/*.proto` | API service definitions |
| **Ent schemas** | `internal/pkg/database/schema/*.go` | Data models |
| **Service logic** | `internal/service/api/*.go` | Business logic implementation |
| **Server setup** | `internal/server/api/web.go` | HTTP server and routes |
| **Configuration** | `config.json` or `config.yaml` | Runtime config |
| **Wire providers** | `*/wire.go` | Dependency injection |
| **Main entry** | `cmd/app/main.go` | Application entry point |

### Generated Files (Never Edit)

| Purpose | Path | Description |
|---------|------|-------------|
| **Proto Go code** | `api/api/v1/*.pb.go` | Generated from proto |
| **Sphere handlers** | `api/api/v1/*.sphere.go` | HTTP routes and handlers |
| **Error code** | `api/api/v1/*_errors.go` | Error definitions |
| **Ent ORM** | `internal/pkg/database/ent/` | Generated ORM code |
| **Wire output** | `cmd/app/wire_gen.go` | Dependency injection code |
| **Swagger docs** | `swagger/docs.go` | API documentation |

### Configuration Files

| File | Purpose |
|------|---------|
| `buf.yaml` | Buf dependency management |
| `buf.gen.yaml` | Protobuf code generation config |
| `Makefile` | Development commands |
| `go.mod` | Go module dependencies |
| `.gitignore` | Git ignore patterns |
| `Dockerfile` | Container build |
| `docker-compose.yaml` | Local deployment |

## Common Patterns Quick Reference

### Adding a New API Endpoint

```bash
# 1. Edit proto file
vim proto/api/v1/myservice.proto

# 2. Generate code
make gen/proto

# 3. Implement service
vim internal/service/api/myservice.go

# 4. Register in server
vim internal/server/api/web.go
# Add: apiv1.RegisterMyServiceHTTPServer(route, w.service)

# 5. Test
make run
curl http://localhost:8899/v1/my/endpoint
```

### Adding a New Data Model

```bash
# 1. Create schema
vim internal/pkg/database/schema/mymodel.go

# 2. Generate Ent code
make gen/db

# 3. Create render function
vim internal/pkg/render/mymodel.go

# 4. Use in service
vim internal/service/api/myservice.go
```

### Adding Authentication to a Route

```go
// In internal/server/api/web.go

// Protected route group
authMiddleware := auth.NewAuthMiddleware[int64, *jwtauth.RBACClaims[int64]](
    w.jwtAuthorizer,
    auth.WithHeaderLoader(auth.AuthorizationHeader),
    auth.WithPrefixTransform(auth.AuthorizationPrefixBearer),
    auth.WithAbortOnError(true),
)
protectedRoute := w.engine.Group("/v1", authMiddleware)

// Register service on protected route
apiv1.RegisterMyServiceHTTPServer(protectedRoute, w.service)
```

## Troubleshooting Guide

### Build and Generation Errors

#### Error: `protoc-gen-sphere: command not found`

**Cause**: Sphere protoc plugins not installed

**Solution**:
```bash
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere@latest
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere-binding@latest
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere-errors@latest

# Verify installation
which protoc-gen-sphere
```

#### Error: `buf: command not found`

**Cause**: Buf CLI not installed

**Solution**:
```bash
# macOS
brew install bufbuild/buf/buf

# Other platforms
go install github.com/bufbuild/buf/cmd/buf@latest
```

#### Error: `wire: command not found`

**Cause**: Wire not installed

**Solution**:
```bash
go install github.com/google/wire/cmd/wire@latest
```

#### Error: `make: *** No rule to make target`

**Cause**: Unsupported make target or wrong directory

**Solution**:
```bash
# Ensure you're in project root
ls Makefile

# List available targets
make help
# Or check Makefile directly
cat Makefile | grep "^[a-z]"
```

### Runtime Errors

#### Error: `bind: address already in use`

**Cause**: Port is already occupied

**Solution**:
```bash
# Find process using port
lsof -i :8899
# Or on Linux
netstat -tulpn | grep 8899

# Kill the process
kill -9 <PID>

# Or change port in config.json
```

#### Error: `failed to connect to database`

**Cause**: Database connection issue

**Solution**:

For SQLite:
```bash
# Ensure directory exists
mkdir -p ./var

# Check permissions
ls -la ./var
```

For MySQL/PostgreSQL:
```bash
# Test connection
mysql -u user -p -h localhost -P 3306 dbname
# Or
psql -h localhost -p 5432 -U user -d dbname

# Check DSN in config.json
# MySQL: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True"
# PostgreSQL: "host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable"
```

#### Error: `undefined: NewService`

**Cause**: Wire providers not generated after adding new dependencies

**Solution**:
```bash
# Regenerate Wire code
make gen/wire

# Check if wire.go includes new providers
cat cmd/app/wire.go
```

#### Error: `no such file or directory: api/api/v1/*.pb.go`

**Cause**: Proto code not generated

**Solution**:
```bash
make gen/proto
```

### API Request Issues

#### Error: `404 Not Found` for valid endpoint

**Cause**: Service not registered or route mismatch

**Solution**:
```bash
# 1. Verify proto HTTP annotation
# Check: option (google.api.http) = {get: "/v1/path"}

# 2. Check service registration
# In internal/server/api/web.go:
# apiv1.RegisterXXXServiceHTTPServer(route, w.service)

# 3. Check server logs for registered routes
make run
# Look for: "Registered route: GET /v1/path"
```

#### Error: `400 Bad Request` with validation error

**Cause**: Request doesn't match proto validation rules

**Solution**:
```bash
# Check proto validation constraints
# Example: [(buf.validate.field).string.min_len = 1]

# Test with valid data
curl -X POST http://localhost:8899/v1/endpoint \
  -H "Content-Type: application/json" \
  -d '{"field": "value"}'
```

#### Error: `401 Unauthorized`

**Cause**: Missing or invalid JWT token

**Solution**:
```bash
# Ensure Authorization header is present
curl http://localhost:8899/v1/protected \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"

# Verify token is valid
# - Not expired
# - Correct signature
# - Correct format: "Bearer <token>"
```

#### Error: `403 Forbidden`

**Cause**: Insufficient permissions

**Solution**:
```go
// Check user role/permissions in token claims
// Verify ACL configuration
// Ensure user has required role or permission
```

### Code Generation Issues

#### Error: Generated code has import errors

**Cause**: Module path mismatch or stale generated code

**Solution**:
```bash
# 1. Clean and regenerate
make clean
make gen/all

# 2. Verify go.mod module path
head -1 go.mod
# Should match import paths in code

# 3. Update dependencies
go mod tidy
```

#### Error: Ent migration fails

**Cause**: Schema changes incompatible with existing data

**Solution**:
```bash
# Development: Drop and recreate database
rm ./var/data.db
make run  # Auto-migration will recreate schema

# Production: Use Atlas migrations
# See: https://entgo.io/docs/versioned-migrations
```

#### Error: Wire build fails with circular dependency

**Cause**: Circular dependency in providers

**Solution**:
```bash
# Check wire output for cycle details
cat cmd/app/wire_gen.go

# Break cycle by:
# 1. Removing circular references
# 2. Using interfaces
# 3. Restructuring dependencies
```

### Proto Definition Issues

#### Error: `unknown field` in proto

**Cause**: Missing import or undefined message

**Solution**:
```protobuf
// Ensure imports are present
import "buf/validate/validate.proto";
import "google/api/annotations.proto";
import "sphere/binding/binding.proto";
import "sphere/errors/errors.proto";

// Check buf.yaml has dependencies
# deps:
#   - buf.build/googleapis/googleapis
#   - buf.build/bufbuild/protovalidate
#   - buf.build/go-sphere/options
```

#### Error: `google/api/annotations.proto: File not found`

**Cause**: Missing buf dependencies

**Solution**:
```bash
# Update buf dependencies
buf mod update

# Or manually add to buf.yaml:
# deps:
#   - buf.build/googleapis/googleapis
```

## Performance Optimization

### Database Query Optimization

```go
// ❌ N+1 Query Problem
for _, user := range users {
    posts, _ := client.User.QueryPosts(user).All(ctx)  // Queries in loop
}

// ✅ Eager Loading
users, err := client.User.Query().
    WithPosts().  // Eager load posts
    All(ctx)

// ✅ Manual Join
var results []struct {
    User *ent.User
    PostCount int
}
client.User.Query().
    GroupBy(user.FieldID).
    Aggregate(ent.Count()).
    Scan(ctx, &results)
```

### Caching Frequent Queries

```go
// Use Sphere cache package
import "github.com/go-sphere/sphere/cache"

// In service
cachedUser, err := s.cache.Get(ctx, fmt.Sprintf("user:%d", userID))
if err == nil {
    return cachedUser.(*ent.User), nil
}

user, err := s.db.User.Get(ctx, userID)
if err != nil {
    return nil, err
}

s.cache.Set(ctx, fmt.Sprintf("user:%d", userID), user, 5*time.Minute)
return user, nil
```

### Connection Pooling

```json
{
  "database": {
    "type": "mysql",
    "dsn": "...",
    "max_open": 25,   // Max open connections
    "max_idle": 5,    // Max idle connections
    "max_lifetime": 300  // Max lifetime in seconds
  }
}
```

## Security Checklist

- [ ] **Secrets**: Never commit secrets to git
- [ ] **JWT Secret**: Use strong random key (32+ characters)
- [ ] **Password**: Always hash with bcrypt
- [ ] **SQL Injection**: Use Ent (parameterized queries)
- [ ] **CORS**: Whitelist specific origins
- [ ] **HTTPS**: Use TLS in production
- [ ] **Headers**: Set security headers (CSP, X-Frame-Options)
- [ ] **Rate Limiting**: Implement rate limits
- [ ] **Input Validation**: Use proto validation rules
- [ ] **Error Messages**: Don't leak sensitive info in errors

## Production Deployment Checklist

- [ ] **Environment Variables**: Use for secrets
- [ ] **Database**: Use PostgreSQL/MySQL, not SQLite
- [ ] **Migrations**: Use versioned migrations
- [ ] **Logging**: Configure structured logging
- [ ] **Monitoring**: Set up health checks
- [ ] **Backups**: Automated database backups
- [ ] **Docker**: Multi-stage build for smaller images
- [ ] **Resource Limits**: Set memory/CPU limits
- [ ] **Graceful Shutdown**: Ensure proper cleanup
- [ ] **Load Balancer**: Use reverse proxy (nginx)

## Environment Variables

### Common Environment Variables

```bash
# Database
export DB_DSN="user:pass@tcp(host:port)/db"

# JWT Secrets
export JWT_SECRET="your-secret-key"
export REFRESH_JWT_SECRET="your-refresh-secret"

# Server Ports
export API_PORT=8899
export DASH_PORT=8800

# Environment
export ENV=production

# Logging
export LOG_LEVEL=info
```

### Using in Config

```json
{
  "database": {
    "dsn": "${DB_DSN}"
  },
  "api": {
    "jwt": "${JWT_SECRET}",
    "http": {
      "address": "0.0.0.0:${API_PORT:8899}"
    }
  }
}
```

## Debugging Tips

### Enable SQL Logging

```json
{
  "database": {
    "debug": true
  }
}
```

### Enable HTTP Request Logging

```go
// In server setup
w.engine.Use(middleware.Logger())
```

### View Generated Routes

```bash
# Check Swagger UI
open http://localhost:8899/swagger/index.html

# Or check server logs on startup
make run | grep "Registered"
```

### Debug Wire Issues

```bash
# Generate with verbose output
cd cmd/app
wire check
wire show
```

## Useful Resources

### Official Documentation

- Sphere: https://github.com/go-sphere/sphere
- Ent: https://entgo.io/docs/getting-started
- Buf: https://buf.build/docs
- Wire: https://github.com/google/wire
- Fiber: https://docs.gofiber.io

### Community

- Sphere Issues: https://github.com/go-sphere/sphere/issues
- Ent Discord: https://discord.gg/ent
- Buf Slack: https://buf.build/links/slack

## Quick Fixes

| Problem | Quick Fix |
|---------|-----------|
| Server won't start | Check port availability, config.json syntax |
| 404 on API | Verify route registration, check HTTP method |
| 401 auth error | Check token format: `Bearer <token>` |
| Database error | Check DSN, ensure DB running, verify schema |
| Import errors | Run `go mod tidy`, check module path |
| Generated code issues | Run `make gen/all`, check proto syntax |
| Wire errors | Verify providers in wire.go, run `make gen/wire` |

## AI Agent Emergency Commands

When something goes wrong, try these in order:

```bash
# 1. Clean and regenerate everything
make clean
go mod tidy
make gen/all

# 2. Restart with fresh database (development only!)
rm -rf ./var
mkdir -p ./var
make run

# 3. Check for errors
make test
make lint

# 4. Verify installations
which protoc buf wire ent
go version

# 5. Update dependencies
go get -u ./...
go mod tidy
```
