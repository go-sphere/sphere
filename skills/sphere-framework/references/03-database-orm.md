# Database and ORM Guide

> **AI Agent Context**: Use this guide when working with databases, defining data models with Ent ORM, or converting between Ent entities and Protobuf messages. This covers the complete database workflow in Sphere.

## Overview

Sphere uses **Ent ORM** as the default database layer:

- **Type-safe**: Code-generated, compile-time type safety
- **Schema-first**: Define schemas in Go, generate migrations
- **Protobuf integration**: Bidirectional conversion with protobuf via `entc-extensions`

## Database Support

Supported databases:
- SQLite (default for development)
- MySQL
- PostgreSQL
- MariaDB

## Complete Workflow: Adding a Data Model

### Step 1: Create Ent Schema

Schemas are defined in `internal/pkg/database/schema/` directory.

**Example: Create a Product schema**

**File: `internal/pkg/database/schema/product.go`**

```go
package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
    "entgo.io/ent/schema/edge"
    "github.com/go-sphere/sphere/idgenerator"
)

// Product holds the schema definition for the Product entity.
type Product struct {
    ent.Schema
}

// Fields of the Product.
func (Product) Fields() []ent.Field {
    return []ent.Field{
        field.Int64("id").
            DefaultFunc(idgenerator.NextId).
            Immutable().
            Comment("Product ID"),

        field.String("name").
            NotEmpty().
            MaxLen(200).
            Comment("Product name"),

        field.Text("description").
            Optional().
            Comment("Product description"),

        field.Int64("price").
            NonNegative().
            Comment("Price in cents"),

        field.Int64("stock").
            Default(0).
            NonNegative().
            Comment("Available stock quantity"),

        field.String("sku").
            Unique().
            NotEmpty().
            Comment("Stock Keeping Unit"),

        field.Enum("status").
            Values("active", "inactive", "discontinued").
            Default("active"),

        field.JSON("metadata", map[string]interface{}{}).
            Optional().
            Comment("Additional metadata"),

        field.Int64("created_at").
            DefaultFunc(func() int64 { return time.Now().Unix() }).
            Immutable(),

        field.Int64("updated_at").
            DefaultFunc(func() int64 { return time.Now().Unix() }).
            UpdateDefault(func() int64 { return time.Now().Unix() }),
    }
}

// Edges of the Product (relationships).
func (Product) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("category", Category.Type).
            Unique().
            Comment("Product category"),

        edge.From("orders", Order.Type).
            Ref("products").
            Comment("Orders containing this product"),
    }
}

// Indexes of the Product.
func (Product) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("name"),
        index.Fields("sku").Unique(),
        index.Fields("status", "created_at"),
    }
}
```

### Step 2: Generate Ent Code

```bash
make gen/db

# Or manually:
cd internal/pkg/database
go run -mod=mod ./cmd/ent generate ./schema
```

**Generated files** (do not edit):
```
internal/pkg/database/ent/
├── product.go              # Product entity
├── product_create.go       # Create operations
├── product_update.go       # Update operations
├── product_delete.go       # Delete operations
├── product_query.go        # Query operations
├── product/                # Field constants
│   └── product.go
└── ...
```

### Step 3: Use in Service Layer

**File: `internal/service/api/product.go`**

```go
package api

import (
    "context"

    "github.com/yourorg/yourproject/internal/pkg/database/ent"
    "github.com/yourorg/yourproject/internal/pkg/database/ent/product"
)

// Create a product
func (s *Service) CreateProduct(ctx context.Context, name, sku string, price int64) (*ent.Product, error) {
    return s.db.Product.
        Create().
        SetName(name).
        SetSKU(sku).
        SetPrice(price).
        SetStock(0).
        SetStatus(product.StatusActive).
        Save(ctx)
}

// Get a product by ID
func (s *Service) GetProduct(ctx context.Context, id int64) (*ent.Product, error) {
    return s.db.Product.
        Query().
        Where(product.ID(id)).
        Only(ctx)
}

// List products with filters
func (s *Service) ListProducts(ctx context.Context, status *string, minPrice *int64) ([]*ent.Product, error) {
    query := s.db.Product.Query()

    // Apply filters
    if status != nil {
        query = query.Where(product.StatusEQ(product.Status(*status)))
    }
    if minPrice != nil {
        query = query.Where(product.PriceGTE(*minPrice))
    }

    return query.
        Order(ent.Desc(product.FieldCreatedAt)).
        All(ctx)
}

// Update a product
func (s *Service) UpdateProduct(ctx context.Context, id int64, name *string, price *int64) (*ent.Product, error) {
    update := s.db.Product.UpdateOneID(id)

    if name != nil {
        update = update.SetName(*name)
    }
    if price != nil {
        update = update.SetPrice(*price)
    }

    return update.Save(ctx)
}

// Delete a product
func (s *Service) DeleteProduct(ctx context.Context, id int64) error {
    return s.db.Product.DeleteOneID(id).Exec(ctx)
}

// Check if product exists
func (s *Service) ProductExists(ctx context.Context, id int64) (bool, error) {
    return s.db.Product.Query().Where(product.ID(id)).Exist(ctx)
}

// Get product with relations
func (s *Service) GetProductWithCategory(ctx context.Context, id int64) (*ent.Product, error) {
    return s.db.Product.
        Query().
        Where(product.ID(id)).
        WithCategory().  // Eager load category
        Only(ctx)
}
```

## Ent Field Types

### Basic Types

```go
field.String("name")           // VARCHAR
field.Text("description")      // TEXT
field.Int("count")             // INT
field.Int64("id")              // BIGINT
field.Int32("quantity")        // INT
field.Float("price")           // FLOAT
field.Bool("active")           // BOOLEAN
field.Time("created_at")       // TIMESTAMP (use int64 for Unix timestamp)
field.Bytes("data")            // BLOB
field.UUID("uuid", uuid.UUID{}) // UUID (requires uuid package)
```

### Special Types

```go
// Enum
field.Enum("status").
    Values("pending", "active", "inactive").
    Default("pending")

// JSON
field.JSON("metadata", map[string]interface{}{})
field.JSON("tags", []string{})

// Array (PostgreSQL only)
field.Strings("tags")  // TEXT[]
field.Ints("scores")   // INT[]
```

### Field Options

```go
field.String("email").
    NotEmpty().           // Cannot be empty string
    Unique().             // Unique constraint
    MaxLen(100).          // Max length
    MinLen(5).            // Min length
    Optional().           // NULL allowed
    Immutable().          // Cannot be updated after creation
    Default("value").     // Default value
    DefaultFunc(func() string { return uuid.New().String() }).
    UpdateDefault(func() time.Time { return time.Now() }).  // Auto-update on every update
    Comment("User email address")

field.Int64("price").
    Positive().           // > 0
    NonNegative().        // >= 0
    Min(0).               // >= 0
    Max(1000000).         // <= 1000000

field.String("name").
    Match(regexp.MustCompile("[a-z]+"))  // Regex validation
```

## Relationships (Edges)

### One-to-One

```go
// User schema
func (User) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("profile", Profile.Type).
            Unique(),  // One-to-one
    }
}

// Profile schema
func (Profile) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("user", User.Type).
            Ref("profile").
            Unique().
            Required(),  // Profile must belong to a user
    }
}

// Usage
profile, err := client.User.
    Query().
    Where(user.ID(uid)).
    QueryProfile().
    Only(ctx)
```

### One-to-Many

```go
// User schema (one user has many posts)
func (User) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("posts", Post.Type),
    }
}

// Post schema (many posts belong to one user)
func (Post) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("author", User.Type).
            Ref("posts").
            Unique().
            Required(),
    }
}

// Usage
posts, err := client.User.
    Query().
    Where(user.ID(uid)).
    QueryPosts().
    All(ctx)
```

### Many-to-Many

```go
// User schema
func (User) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("groups", Group.Type),
    }
}

// Group schema
func (Group) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("users", User.Type).
            Ref("groups"),
    }
}

// Usage
groups, err := client.User.
    Query().
    Where(user.ID(uid)).
    QueryGroups().
    All(ctx)

// Reverse query
users, err := client.Group.
    Query().
    Where(group.ID(gid)).
    QueryUsers().
    All(ctx)
```

## Query Patterns

### Basic Queries

```go
// Get by ID
user, err := client.User.Get(ctx, id)

// Get first
user, err := client.User.Query().First(ctx)

// Get by field
user, err := client.User.Query().
    Where(user.Email("test@example.com")).
    Only(ctx)

// List all
users, err := client.User.Query().All(ctx)

// Count
count, err := client.User.Query().Count(ctx)

// Exist
exists, err := client.User.Query().
    Where(user.Email("test@example.com")).
    Exist(ctx)
```

### Filtering

```go
// Simple filters
users, err := client.User.Query().
    Where(user.Age(30)).
    All(ctx)

// Comparison
users, err := client.User.Query().
    Where(
        user.AgeGT(18),      // >
        user.AgeLT(65),      // <
        user.AgeGTE(18),     // >=
        user.AgeLTE(65),     // <=
        user.AgeNEQ(25),     // !=
    ).
    All(ctx)

// IN query
users, err := client.User.Query().
    Where(user.IDIn(1, 2, 3, 4)).
    All(ctx)

// NOT IN
users, err := client.User.Query().
    Where(user.IDNotIn(1, 2, 3)).
    All(ctx)

// String contains
users, err := client.User.Query().
    Where(user.NameContains("john")).
    All(ctx)

// String prefix/suffix
users, err := client.User.Query().
    Where(
        user.NameHasPrefix("Dr."),
        user.EmailHasSuffix("@gmail.com"),
    ).
    All(ctx)

// NULL checks
users, err := client.User.Query().
    Where(user.BioIsNil()).
    All(ctx)

users, err := client.User.Query().
    Where(user.BioNotNil()).
    All(ctx)
```

### Logical Operators

```go
// AND (default)
users, err := client.User.Query().
    Where(
        user.AgeGT(18),
        user.StatusEQ(user.StatusActive),
    ).
    All(ctx)

// OR
users, err := client.User.Query().
    Where(
        user.Or(
            user.StatusEQ(user.StatusActive),
            user.StatusEQ(user.StatusPending),
        ),
    ).
    All(ctx)

// NOT
users, err := client.User.Query().
    Where(
        user.Not(user.Status(user.StatusBanned)),
    ).
    All(ctx)

// Complex
users, err := client.User.Query().
    Where(
        user.And(
            user.AgeGT(18),
            user.Or(
                user.CountryEQ("US"),
                user.CountryEQ("CA"),
            ),
        ),
    ).
    All(ctx)
```

### Sorting and Pagination

```go
// Order by
users, err := client.User.Query().
    Order(ent.Asc(user.FieldName)).
    All(ctx)

users, err := client.User.Query().
    Order(ent.Desc(user.FieldCreatedAt)).
    All(ctx)

// Multiple orders
users, err := client.User.Query().
    Order(
        ent.Desc(user.FieldAge),
        ent.Asc(user.FieldName),
    ).
    All(ctx)

// Pagination
users, err := client.User.Query().
    Offset(20).   // Skip first 20
    Limit(10).    // Take 10
    All(ctx)

// Cursor-based pagination
users, err := client.User.Query().
    Where(user.IDGT(lastSeenID)).
    Limit(10).
    All(ctx)
```

### Selecting Fields

```go
// Select specific fields
var v []struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

err := client.User.Query().
    Select(user.FieldName, user.FieldEmail).
    Scan(ctx, &v)

// Group by
var v []struct {
    Status string `json:"status"`
    Count  int    `json:"count"`
}

err := client.User.Query().
    GroupBy(user.FieldStatus).
    Aggregate(ent.Count()).
    Scan(ctx, &v)
```

### Aggregations

```go
// Count
count, err := client.Product.Query().
    Where(product.StatusEQ(product.StatusActive)).
    Count(ctx)

// Sum
sum, err := client.Product.Query().
    Aggregate(ent.Sum(product.FieldPrice)).
    Int(ctx)

// Average
avg, err := client.Product.Query().
    Aggregate(ent.Mean(product.FieldPrice)).
    Float64(ctx)

// Min/Max
max, err := client.Product.Query().
    Aggregate(ent.Max(product.FieldPrice)).
    Int(ctx)
```

## Transactions

```go
func (s *Service) TransferStock(ctx context.Context, fromID, toID int64, qty int64) error {
    tx, err := s.db.Tx(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Decrease stock from source
    err = tx.Product.
        UpdateOneID(fromID).
        AddStock(-qty).
        Exec(ctx)
    if err != nil {
        return err
    }

    // Increase stock to destination
    err = tx.Product.
        UpdateOneID(toID).
        AddStock(qty).
        Exec(ctx)
    if err != nil {
        return err
    }

    // Commit transaction
    return tx.Commit()
}
```

## Protobuf Integration (Render Layer)

### Convert Ent to Protobuf

**File: `internal/pkg/render/product.go`**

```go
package render

import (
    apiv1 "github.com/yourorg/yourproject/api/api/v1"
    "github.com/yourorg/yourproject/internal/pkg/database/ent"
)

// Product converts ent.Product to protobuf Product
func (r *Render) Product(p *ent.Product) *apiv1.Product {
    if p == nil {
        return nil
    }

    return &apiv1.Product{
        Id:          p.ID,
        Name:        p.Name,
        Description: p.Description,
        Price:       p.Price,
        Stock:       p.Stock,
        Sku:         p.SKU,
        Status:      string(p.Status),
        CreatedAt:   p.CreatedAt,
        UpdatedAt:   p.UpdatedAt,
    }
}

// Products converts slice of ent.Product to protobuf slice
func (r *Render) Products(products []*ent.Product) []*apiv1.Product {
    result := make([]*apiv1.Product, len(products))
    for i, p := range products {
        result[i] = r.Product(p)
    }
    return result
}
```

## Database Configuration

### Config Structure

**File: `internal/config/config.go`**

```go
type Config struct {
    Database *client.Config `json:"database"`
}
```

**File: `internal/pkg/database/client/config.go`**

```go
type Config struct {
    Type     string `json:"type"`      // sqlite3, mysql, postgres
    DSN      string `json:"dsn"`       // Connection string
    Path     string `json:"path"`      // SQLite path
    Debug    bool   `json:"debug"`     // Enable SQL logging
    MaxOpen  int    `json:"max_open"`  // Max open connections
    MaxIdle  int    `json:"max_idle"`  // Max idle connections
}
```

### Configuration Examples

**SQLite (Development)**:
```json
{
  "database": {
    "type": "sqlite3",
    "path": "file:./var/data.db?_fk=1",
    "debug": true
  }
}
```

**MySQL (Production)**:
```json
{
  "database": {
    "type": "mysql",
    "dsn": "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local",
    "debug": false,
    "max_open": 25,
    "max_idle": 5
  }
}
```

**PostgreSQL**:
```json
{
  "database": {
    "type": "postgres",
    "dsn": "host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable",
    "debug": false
  }
}
```

## Database Migrations

Ent supports automatic migrations:

```go
// In initialization code (internal/biz/dash_initialize.go)
func (d *DashInitialize) Boot(ctx context.Context) error {
    // Run auto-migration
    if err := d.db.Schema.Create(ctx); err != nil {
        return fmt.Errorf("failed creating schema: %w", err)
    }

    // ... rest of initialization
}
```

**IMPORTANT**: Auto-migration is safe for development, but use versioned migrations in production (see Ent Atlas documentation).

## AI Agent Checklist

When working with database models:

- [ ] Schema file created in `internal/pkg/database/schema/`
- [ ] Fields defined with appropriate types and constraints
- [ ] Indexes added for frequently queried fields
- [ ] Edges (relationships) defined correctly
- [ ] `make gen/db` executed successfully
- [ ] Render functions created in `internal/pkg/render/`
- [ ] Service layer methods use type-safe queries
- [ ] Errors handled (check `ent.IsNotFound(err)`)
- [ ] Transactions used for multi-step operations
- [ ] Database configuration updated in `config.json`
