# Getting Started with Sphere

> **AI Agent Context**: Use this guide when the user wants to create a new Sphere project or understand the initial project setup. This covers project creation, directory structure, configuration, and first run.

## Prerequisites

Before creating a Sphere project, ensure these are installed:

```bash
# Required
go >= 1.22              # Go language
protoc                  # Protocol Buffer compiler
buf                     # Modern protobuf tool
ent                     # Ent ORM code generator

# Optional but recommended
wire                    # Dependency injection
swag                    # Swagger doc generator
make                    # Build automation
docker                  # Containerization
```

**Installation references**:
- Go: https://golang.org/dl/
- Protoc: https://grpc.io/docs/protoc-installation/
- Buf: https://buf.build/docs/installation
- Ent: `go install entgo.io/ent/cmd/ent@latest`
- Wire: `go install github.com/google/wire/cmd/wire@latest`

## Creating a New Project

### Option 1: Using sphere-cli (Recommended)

```bash
# Install sphere CLI
go install github.com/go-sphere/sphere-cli@latest

# List available templates
sphere-cli create list

# Create new project
sphere-cli create --name my-project --module github.com/yourorg/my-project

# Or with specific template
sphere-cli create --name my-project --module github.com/yourorg/my-project --layout simple

# Change to project directory
cd my-project

# Initialize dependencies
go mod tidy

# Generate all code
make gen/all

# Run the application
make run
```

**What `sphere-cli create` does:**
1. Clones the specified template (default: sphere-layout)
2. Renames the project directory
3. Updates go.mod with your module name
4. Replaces all import paths in Go files

**Available templates:**
- `layout` or empty: Full-featured template (default)
- `simple`: Minimal template
- `bun`: Bun ORM instead of Ent
- Custom: Any git URL

### Option 2: Manual Setup from Template

If you want to customize the template or sphere-cli is unavailable:

```bash
# Clone one of the official templates
git clone https://github.com/go-sphere/sphere-layout.git my-project
cd my-project

# Remove git history
rm -rf .git
git init

# Update module name
# Edit go.mod and replace "github.com/go-sphere/sphere-layout" with your module name

# Find and replace import paths
find . -type f -name "*.go" -exec sed -i '' 's|github.com/go-sphere/sphere-layout|your-module-name|g' {} +

# Initialize dependencies
go mod tidy

# Generate code
make gen/all
```

## Understanding the Generated Project

### Directory Structure Explained

```
my-project/
├── cmd/app/                  # ENTRY POINT - Application binary
│   ├── main.go               # Main function, calls app.Execute()
│   ├── builder.go            # Application builder, wires components
│   ├── wire.go               # Wire dependency injection declarations
│   └── wire_gen.go           # [GENERATED] Wire output
│
├── proto/                    # SOURCE - Protocol Buffer definitions
│   ├── api/v1/               # API service definitions
│   │   ├── auth.proto        # Example: Authentication service
│   │   └── user.proto        # Example: User service
│   ├── common/               # Shared messages and enums
│   ├── errors/               # Error definitions
│   └── buf.yaml              # Buf configuration
│
├── api/                      # [GENERATED] - Go code from proto
│   └── api/v1/               # Generated packages
│       ├── *.pb.go           # Protobuf message code
│       ├── *.sphere.go       # HTTP handlers and routes
│       └── *_errors.go       # Error definitions
│
├── internal/                 # IMPLEMENTATION - Your code here
│   ├── config/               # Configuration management
│   │   ├── config.go         # Config struct definition
│   │   ├── load.go           # Config loading logic
│   │   └── wire.go           # Config providers
│   │
│   ├── server/               # HTTP servers
│   │   ├── api/              # Public API server
│   │   │   ├── web.go        # Server setup and route registration
│   │   │   └── middleware.go # API-specific middleware
│   │   ├── dash/             # Admin dashboard server
│   │   └── fileserver/       # File serving server
│   │
│   ├── service/              # Business logic (implements proto services)
│   │   ├── api/              # API service implementations
│   │   │   ├── service.go    # Service struct with dependencies
│   │   │   ├── auth.go       # Auth service methods
│   │   │   └── user.go       # User service methods
│   │   └── dash/             # Dashboard service implementations
│   │
│   ├── biz/                  # Lifecycle tasks
│   │   ├── dash_initialize.go # Initial data setup
│   │   └── connect_cleaner.go # Cleanup on shutdown
│   │
│   └── pkg/                  # Internal packages
│       ├── database/         # Database layer
│       │   ├── client/       # DB client factory
│       │   ├── schema/       # [SOURCE] Ent schemas
│       │   └── ent/          # [GENERATED] Ent ORM code
│       ├── dao/              # Data Access Objects
│       ├── render/           # ent → protobuf conversion
│       ├── conv/             # Data conversion utilities
│       └── auth/             # Auth logic
│
├── devops/                   # Deployment
│   ├── Dockerfile            # Container definition
│   └── docker-compose.yaml   # Local development setup
│
├── swagger/                  # [GENERATED] API documentation
│   └── docs.go               # Swagger definitions
│
├── config.json               # Runtime configuration
├── Makefile                  # Development commands
├── go.mod                    # Go dependencies
└── README.md                 # Project documentation
```

**Key Principle**: Edit files in `proto/`, `internal/pkg/database/schema/`, and `internal/service/` directories. Everything else is either generated or configuration.

## Initial Configuration

### Configuration File (config.json)

The default configuration file is located at the project root:

```json
{
  "log": {
    "level": "info",
    "file": {
      "enable": true,
      "path": "./var/logs"
    },
    "console": {
      "enable": true
    }
  },
  "database": {
    "type": "sqlite3",
    "path": "file:./var/data.db?_fk=1"
  },
  "dash": {
    "auth_jwt": "your-secret-key-change-me",
    "refresh_jwt": "your-refresh-secret-change-me",
    "http": {
      "address": "0.0.0.0:8800"
    }
  },
  "api": {
    "jwt": "your-api-secret-change-me",
    "http": {
      "address": "0.0.0.0:8899",
      "cors": ["http://localhost:3000"]
    }
  },
  "file": {
    "address": "0.0.0.0:9900",
    "cors": ["*"]
  },
  "storage": {
    "root_dir": "./var/file",
    "public_base": "http://localhost:9900"
  }
}
```

**IMPORTANT Configuration Notes:**

1. **JWT Secrets**: Change all `*_jwt` values in production
2. **Database**: Default is SQLite, can switch to MySQL/PostgreSQL
3. **CORS**: Adjust `cors` arrays for your frontend domains
4. **Ports**:
   - API: 8899
   - Dashboard: 8800
   - File Server: 9900

### Environment Variable Support

Configuration values can use environment variables:

```json
{
  "database": {
    "type": "mysql",
    "dsn": "${DB_DSN:root:password@tcp(localhost:3306)/mydb}"
  },
  "api": {
    "jwt": "${API_JWT_SECRET}"
  }
}
```

Syntax: `${VAR_NAME:default_value}` or `${VAR_NAME}`

## First Run

### Step 1: Generate All Code

```bash
# This runs: gen/db, gen/proto, gen/docs, gen/wire
make gen/all
```

**What happens:**
- `gen/db`: Generates Ent ORM code from `internal/pkg/database/schema/`
- `gen/proto`: Generates Go code from `.proto` files
- `gen/docs`: Generates Swagger documentation
- `gen/wire`: Generates dependency injection code

**Expected output:**
```
✓ Ent generation complete
✓ Protocol Buffer generation complete
✓ Swagger docs generated
✓ Wire code generated
```

### Step 2: Run the Application

```bash
make run
```

**Expected output:**
```
INFO    Starting application
INFO    Database connected: sqlite3
INFO    API Server listening on: http://0.0.0.0:8899
INFO    Dashboard Server listening on: http://0.0.0.0:8800
INFO    File Server listening on: http://0.0.0.0:9900
```

### Step 3: Verify It's Working

```bash
# Check API health
curl http://localhost:8899/health

# Response:
# {"status": "ok"}

# View API documentation
open http://localhost:8899/swagger/index.html

# View Dashboard (if assets are built)
open http://localhost:8800
```

## Understanding the Application Lifecycle

### Startup Sequence

```
1. main.go: app.Execute(NewApplication)
   │
2. builder.go: NewApplication (Wire builds dependencies)
   │
   ├─ Load config.json
   ├─ Connect to database
   ├─ Initialize cache
   ├─ Create all servers
   └─ Create lifecycle tasks
   │
3. app.Execute():
   │
   ├─ Phase 1: Boot (tasks.Boot())
   │   └─ Run initialization tasks (e.g., create admin user)
   │
   ├─ Phase 2: Start (servers.Start())
   │   └─ Start all HTTP servers concurrently
   │
   └─ Phase 3: Wait for signal (Ctrl+C)
       │
       └─ Phase 4: Shutdown (servers.Stop(), tasks.Stop())
           └─ Graceful shutdown with 30s timeout
```

### Lifecycle Interfaces

**Bootable** (for one-time initialization tasks):
```go
type Bootable interface {
    Boot(ctx context.Context) error
}
```

**Server** (for long-running services):
```go
type Server interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

## Common Initial Tasks

### Task 1: Change Module Name

If you cloned manually, update all import paths:

```bash
# Edit go.mod
module your-module-name

# Replace in all Go files
find . -type f -name "*.go" -exec sed -i '' 's|github.com/go-sphere/sphere-layout|your-module-name|g' {} +

# Re-download dependencies
go mod tidy
```

### Task 2: Update Configuration

Edit `config.json`:

```json
{
  "api": {
    "jwt": "your-strong-secret-here",
    "http": {
      "cors": ["https://your-frontend-domain.com"]
    }
  }
}
```

### Task 3: Add Your First API

See `02-api-development.md` for detailed steps:

1. Define proto service in `proto/api/v1/yourservice.proto`
2. Run `make gen/proto`
3. Implement service in `internal/service/api/yourservice.go`
4. Run `make gen/wire` (if new service struct added)
5. Run `make run`

## Project Templates

Sphere provides different templates for different needs:

### 1. sphere-layout (Full-Featured)

**Use when:** Building production applications with dashboard, file storage, multi-tenancy

**Includes:**
- API server + Dashboard server + File server
- Ent ORM with example models
- JWT authentication with RBAC
- File storage and caching
- Docker and CI/CD setup
- Admin dashboard frontend (optional)

### 2. sphere-simple-layout (Minimal)

**Use when:** Building simple APIs without dashboard or complex auth

**Includes:**
- Single API server
- Basic authentication
- Minimal dependencies
- Faster startup

### 3. sphere-bun-layout (Alternative ORM)

**Use when:** Prefer Bun ORM over Ent

**Includes:**
- Bun ORM instead of Ent
- Similar structure to sphere-layout
- PostgreSQL-focused

**Command:**
```bash
sphere create my-project --template simple
sphere create my-project --template bun
```

## Troubleshooting Initial Setup

### Issue: `make gen/proto` fails

**Error:** `protoc-gen-sphere: command not found`

**Solution:**
```bash
# Install Sphere protoc plugins
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere@latest
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere-binding@latest
go install github.com/go-sphere/sphere/cmd/protoc-gen-sphere-errors@latest

# Verify installation
which protoc-gen-sphere
```

### Issue: `make gen/wire` fails

**Error:** `wire: command not found`

**Solution:**
```bash
go install github.com/google/wire/cmd/wire@latest
```

### Issue: Port already in use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using the port
lsof -i :8899

# Kill the process
kill -9 <PID>

# Or change the port in config.json
```

### Issue: Database connection error

**Error:** `failed to connect to database`

**Solution:**
```bash
# For SQLite, ensure directory exists
mkdir -p ./var

# For MySQL/PostgreSQL, check DSN in config.json
# Format: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True"
```

## Next Steps

Now that your project is set up:

1. **Define your first API** → See `02-api-development.md`
2. **Add database models** → See `03-database-orm.md`
3. **Set up authentication** → See `04-auth-permissions.md`

## AI Agent Checklist

When creating a new Sphere project, ensure:

- [ ] Go 1.22+ is installed
- [ ] Protoc and Buf are installed
- [ ] Project created with `sphere create` or template cloned
- [ ] Module name updated in go.mod and imports
- [ ] `make gen/all` runs successfully
- [ ] `config.json` reviewed and JWT secrets changed
- [ ] `make run` starts all servers
- [ ] Health endpoint responds: `curl http://localhost:8899/health`
- [ ] User understands the directory structure (proto/, internal/, api/)
