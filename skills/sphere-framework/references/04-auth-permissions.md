# Authentication and Authorization Guide

> **AI Agent Context**: Use this guide when implementing authentication (who are you) and authorization (what can you do) in Sphere applications. Covers JWT tokens, RBAC, ACL, middleware, and multi-platform authentication.

## Overview

Sphere provides built-in authentication and authorization through:

- **JWT Authentication**: Token-based auth with configurable claims
- **RBAC (Role-Based Access Control)**: User roles and permissions
- **ACL (Access Control List)**: Fine-grained permission checks
- **Multi-Platform Auth**: Support for multiple login methods (email, OAuth, WeChat, etc.)
- **Auth Middleware**: Route protection and context injection

## Architecture

```
┌──────────────┐
│ User Login   │  (Email, OAuth, WeChat Mini, etc.)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Auth Helper  │  auth.Auth() - handles login/register
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Generate JWT │  authorizer.GenerateToken()
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Return Token │  Client stores token
└──────────────┘


┌──────────────┐
│ API Request  │  Authorization: Bearer <token>
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Auth MW      │  Validates token, extracts claims
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ ACL Check    │  Verify user has required permission
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Handler      │  Access user info from context
└──────────────┘
```

## JWT Authentication

### JWT Authorizer Setup

**File: `internal/pkg/auth/jwt.go`** (or in server setup)

```go
import (
    "github.com/go-sphere/sphere/auth/jwtauth"
)

// Create JWT authorizer with RBAC claims
jwtAuthorizer := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](
    &jwtauth.Config{
        Secret:         "your-secret-key",
        Expired:        3600,  // 1 hour in seconds
        RefreshExpired: 86400, // 24 hours in seconds
        Issuer:         "your-app-name",
    },
)
```

### JWT Claims Structure

**Built-in RBAC Claims:**

```go
type RBACClaims[ID comparable] struct {
    UserID   ID       `json:"user_id"`
    Role     string   `json:"role"`
    Platform string   `json:"platform"`
    jwt.RegisteredClaims
}
```

**Custom Claims:**

```go
type CustomClaims struct {
    UserID    int64    `json:"user_id"`
    Email     string   `json:"email"`
    Role      string   `json:"role"`
    Permissions []string `json:"permissions"`
    jwt.RegisteredClaims
}

// Create authorizer with custom claims
authorizer := jwtauth.NewJwtAuth[CustomClaims](config)
```

### Generating Tokens

```go
import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

// Generate access token
func (s *Service) GenerateAccessToken(userID int64, role, platform string) (string, error) {
    claims := &jwtauth.RBACClaims[int64]{
        UserID:   userID,
        Role:     role,
        Platform: platform,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "your-app",
        },
    }

    return s.authorizer.GenerateToken(context.Background(), claims)
}

// Generate refresh token
func (s *Service) GenerateRefreshToken(userID int64) (string, error) {
    claims := &jwtauth.RBACClaims[int64]{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "your-app",
        },
    }

    return s.authorizer.GenerateRefreshToken(context.Background(), claims)
}
```

### Validating Tokens

```go
// Validate and parse token
claims, err := authorizer.ValidateToken(ctx, tokenString)
if err != nil {
    // Invalid or expired token
    return err
}

// Access claims
userID := claims.UserID
role := claims.Role
```

## Authentication Middleware

### Basic Auth Middleware

**File: `internal/server/api/web.go`**

```go
import (
    "github.com/go-sphere/sphere/auth"
    "github.com/go-sphere/sphere/auth/jwtauth"
)

func (w *Web) Start(ctx context.Context) error {
    // Create auth middleware
    authMiddleware := auth.NewAuthMiddleware[int64, *jwtauth.RBACClaims[int64]](
        w.jwtAuthorizer,
        auth.WithHeaderLoader(auth.AuthorizationHeader),         // Load from "Authorization" header
        auth.WithPrefixTransform(auth.AuthorizationPrefixBearer), // Strip "Bearer " prefix
        auth.WithAbortOnError(true),                              // Return 401 if auth fails
    )

    // Apply to routes
    publicRoute := w.engine.Group("/v1")
    protectedRoute := w.engine.Group("/v1", authMiddleware)

    // Public endpoints
    apiv1.RegisterAuthServiceHTTPServer(publicRoute, w.service)

    // Protected endpoints
    apiv1.RegisterUserServiceHTTPServer(protectedRoute, w.service)

    return w.engine.Start(ctx)
}
```

### Accessing Auth Context in Handlers

```go
import (
    "github.com/go-sphere/sphere/auth"
)

func (s *Service) GetCurrentUser(ctx context.Context, req *apiv1.GetCurrentUserRequest) (*apiv1.User, error) {
    // Extract claims from context
    claims, ok := auth.FromContext[*jwtauth.RBACClaims[int64]](ctx)
    if !ok {
        return nil, errors.New("unauthorized")
    }

    userID := claims.UserID
    role := claims.Role

    // Get user from database
    user, err := s.db.User.Get(ctx, userID)
    if err != nil {
        return nil, err
    }

    return s.render.User(user), nil
}
```

## Multi-Platform Authentication

### Auth Helper

Sphere provides an `auth.Auth()` helper for unified authentication logic:

```go
import (
    "github.com/go-sphere/sphere/auth"
)

// Auth modes
const (
    CreateWithoutCheck  // Create user if not exists, no error if exists
    CreateOrError       // Error if user already exists
    LoginOnly           // Error if user doesn't exist
    LoginOrCreate       // Create user if not exists
)

// Example: WeChat Mini Program login
func (s *Service) AuthWithWxMini(ctx context.Context, req *apiv1.AuthWithWxMiniRequest) (*apiv1.AuthResponse, error) {
    // 1. Get user info from WeChat API
    session, err := s.wechat.JsCode2Session(ctx, req.Code)
    if err != nil {
        return nil, err
    }

    // 2. Use auth helper to login or register
    result, err := auth.Auth(ctx, s.db, session.OpenID, auth.PlatformWechatMini,
        auth.WithAuthMode(auth.CreateWithoutCheck),
        auth.WithOnCreateUser(func(create *ent.UserCreate) *ent.UserCreate {
            return create.
                SetUsername(fmt.Sprintf("wx_%s", generateRandomID())).
                SetAvatar("default-avatar.png")
        }),
    )
    if err != nil {
        return nil, err
    }

    // 3. Generate JWT token
    token, err := s.authorizer.GenerateToken(ctx, &jwtauth.RBACClaims[int64]{
        UserID:   result.User.ID,
        Role:     result.User.Role,
        Platform: string(auth.PlatformWechatMini),
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    })
    if err != nil {
        return nil, err
    }

    return &apiv1.AuthResponse{
        Token: token,
        User:  s.render.User(result.User),
        IsNew: result.IsNew,
    }, nil
}
```

### Supported Platforms

```go
const (
    PlatformEmail       Platform = "email"
    PlatformPhone       Platform = "phone"
    PlatformWechat      Platform = "wechat"
    PlatformWechatMini  Platform = "wechat_mini"
    PlatformApple       Platform = "apple"
    PlatformGoogle      Platform = "google"
    PlatformGithub      Platform = "github"
)
```

### Database Schema for Multi-Platform

**File: `internal/pkg/database/schema/user_platform.go`**

```go
type UserPlatform struct {
    ent.Schema
}

func (UserPlatform) Fields() []ent.Field {
    return []ent.Field{
        field.Int64("id").DefaultFunc(idgenerator.NextId),
        field.Int64("user_id").Comment("Associated user ID"),
        field.String("platform").Comment("Platform type (wechat_mini, email, etc.)"),
        field.String("platform_id").Comment("Platform-specific user ID (openid, email, etc.)"),
        field.JSON("extra_data", map[string]interface{}{}).Optional(),
        field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().Unix() }),
    }
}

func (UserPlatform) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("user", User.Type).
            Ref("platforms").
            Unique().
            Required().
            Field("user_id"),
    }
}

func (UserPlatform) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("platform", "platform_id").Unique(),
    }
}
```

## Role-Based Access Control (RBAC)

### Define Roles

**File: `internal/pkg/auth/role.go`**

```go
package auth

const (
    RoleAdmin     = "admin"
    RoleModerator = "moderator"
    RoleUser      = "user"
    RoleGuest     = "guest"
)

// Check if user has required role
func HasRole(userRole string, requiredRole string) bool {
    roleHierarchy := map[string]int{
        RoleAdmin:     4,
        RoleModerator: 3,
        RoleUser:      2,
        RoleGuest:     1,
    }

    return roleHierarchy[userRole] >= roleHierarchy[requiredRole]
}
```

### Role Middleware

```go
// Create role-checking middleware
func (w *Web) withRole(requiredRole string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := auth.FromContext[*jwtauth.RBACClaims[int64]](c.UserContext())
        if !ok {
            return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
        }

        if !HasRole(claims.Role, requiredRole) {
            return fiber.NewError(fiber.StatusForbidden, "insufficient permissions")
        }

        return c.Next()
    }
}

// Apply to routes
adminRoute := protectedRoute.Group("/admin", w.withRole(auth.RoleAdmin))
apiv1.RegisterAdminServiceHTTPServer(adminRoute, w.service)
```

## Access Control List (ACL)

### ACL Setup

**File: `internal/server/dash/acl.go`**

```go
import (
    "github.com/go-sphere/sphere/acl"
)

// Define permissions
const (
    PermissionUserRead   = "user:read"
    PermissionUserWrite  = "user:write"
    PermissionUserDelete = "user:delete"
    PermissionAdmin      = "admin"
)

// Create ACL
func NewACL() *acl.ACL {
    a := acl.NewACL()

    // Admin has all permissions
    a.Allow(acl.PermissionAll, RoleAdmin)

    // Moderator permissions
    a.Allow(PermissionUserRead, RoleModerator)
    a.Allow(PermissionUserWrite, RoleModerator)

    // User permissions
    a.Allow(PermissionUserRead, RoleUser)

    return a
}
```

### Permission Middleware

```go
func (w *Web) withPermission(permission string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := auth.FromContext[*jwtauth.RBACClaims[int64]](c.UserContext())
        if !ok {
            return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
        }

        // Check permission
        if !w.acl.Can(permission, claims.Role) {
            return fiber.NewError(fiber.StatusForbidden, "permission denied")
        }

        return c.Next()
    }
}

// Apply to specific routes
userRoute := protectedRoute.Group("/users")
userRoute.Get("/", w.withPermission(PermissionUserRead), getUsersHandler)
userRoute.Post("/", w.withPermission(PermissionUserWrite), createUserHandler)
userRoute.Delete("/:id", w.withPermission(PermissionUserDelete), deleteUserHandler)
```

## Complete Authentication Flow Example

### 1. Define Auth Proto

**File: `proto/api/v1/auth.proto`**

```protobuf
service AuthService {
  rpc Login(LoginRequest) returns (AuthResponse) {
    option (google.api.http) = {
      post: "/v1/auth/login"
      body: "*"
    };
  }

  rpc Register(RegisterRequest) returns (AuthResponse) {
    option (google.api.http) = {
      post: "/v1/auth/register"
      body: "*"
    };
  }

  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse) {
    option (google.api.http) = {
      post: "/v1/auth/refresh"
      body: "*"
    };
  }

  rpc GetCurrentUser(GetCurrentUserRequest) returns (User) {
    option (google.api.http) = {
      get: "/v1/auth/me"
    };
  }
}

message LoginRequest {
  string email = 1;
  string password = 2;
}

message RegisterRequest {
  string email = 1;
  string password = 2;
  string username = 3;
}

message AuthResponse {
  string access_token = 1;
  string refresh_token = 2;
  User user = 3;
}

message RefreshTokenRequest {
  string refresh_token = 1;
}

message RefreshTokenResponse {
  string access_token = 1;
}

message GetCurrentUserRequest {}
```

### 2. Implement Auth Service

**File: `internal/service/api/auth.go`**

```go
package api

import (
    "context"
    "time"

    "github.com/go-sphere/sphere/auth"
    apiv1 "github.com/yourorg/yourproject/api/api/v1"
    "golang.org/x/crypto/bcrypt"
)

func (s *Service) Register(ctx context.Context, req *apiv1.RegisterRequest) (*apiv1.AuthResponse, error) {
    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        return nil, err
    }

    // Use auth helper
    result, err := auth.Auth(ctx, s.db, req.Email, auth.PlatformEmail,
        auth.WithAuthMode(auth.CreateOrError),
        auth.WithOnCreateUser(func(create *ent.UserCreate) *ent.UserCreate {
            return create.
                SetUsername(req.Username).
                SetEmail(req.Email).
                SetPassword(string(hashedPassword)).
                SetRole("user")
        }),
    )
    if err != nil {
        return nil, err
    }

    // Generate tokens
    accessToken, err := s.generateAccessToken(result.User)
    if err != nil {
        return nil, err
    }

    refreshToken, err := s.generateRefreshToken(result.User)
    if err != nil {
        return nil, err
    }

    return &apiv1.AuthResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        User:         s.render.User(result.User),
    }, nil
}

func (s *Service) Login(ctx context.Context, req *apiv1.LoginRequest) (*apiv1.AuthResponse, error) {
    // Find user by email
    user, err := s.db.User.Query().
        Where(user.EmailEQ(req.Email)).
        Only(ctx)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, errors.New("invalid credentials")
        }
        return nil, err
    }

    // Verify password
    err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
    if err != nil {
        return nil, errors.New("invalid credentials")
    }

    // Generate tokens
    accessToken, err := s.generateAccessToken(user)
    if err != nil {
        return nil, err
    }

    refreshToken, err := s.generateRefreshToken(user)
    if err != nil {
        return nil, err
    }

    return &apiv1.AuthResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        User:         s.render.User(user),
    }, nil
}

func (s *Service) RefreshToken(ctx context.Context, req *apiv1.RefreshTokenRequest) (*apiv1.RefreshTokenResponse, error) {
    // Validate refresh token
    claims, err := s.authorizer.ValidateRefreshToken(ctx, req.RefreshToken)
    if err != nil {
        return nil, errors.New("invalid refresh token")
    }

    // Get user
    user, err := s.db.User.Get(ctx, claims.UserID)
    if err != nil {
        return nil, err
    }

    // Generate new access token
    accessToken, err := s.generateAccessToken(user)
    if err != nil {
        return nil, err
    }

    return &apiv1.RefreshTokenResponse{
        AccessToken: accessToken,
    }, nil
}

func (s *Service) GetCurrentUser(ctx context.Context, req *apiv1.GetCurrentUserRequest) (*apiv1.User, error) {
    claims, ok := auth.FromContext[*jwtauth.RBACClaims[int64]](ctx)
    if !ok {
        return nil, errors.New("unauthorized")
    }

    user, err := s.db.User.Get(ctx, claims.UserID)
    if err != nil {
        return nil, err
    }

    return s.render.User(user), nil
}

// Helper methods
func (s *Service) generateAccessToken(user *ent.User) (string, error) {
    claims := &jwtauth.RBACClaims[int64]{
        UserID:   user.ID,
        Role:     user.Role,
        Platform: "email",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    return s.authorizer.GenerateToken(context.Background(), claims)
}

func (s *Service) generateRefreshToken(user *ent.User) (string, error) {
    claims := &jwtauth.RBACClaims[int64]{
        UserID: user.ID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    return s.authorizer.GenerateRefreshToken(context.Background(), claims)
}
```

### 3. Register Routes

**File: `internal/server/api/web.go`**

```go
func (w *Web) Start(ctx context.Context) error {
    // Public routes (no auth required)
    publicRoute := w.engine.Group("/v1")
    apiv1.RegisterAuthServiceHTTPServer(publicRoute, w.service)

    // Protected routes (auth required)
    authMiddleware := auth.NewAuthMiddleware[int64, *jwtauth.RBACClaims[int64]](
        w.jwtAuthorizer,
        auth.WithHeaderLoader(auth.AuthorizationHeader),
        auth.WithPrefixTransform(auth.AuthorizationPrefixBearer),
        auth.WithAbortOnError(true),
    )
    protectedRoute := w.engine.Group("/v1", authMiddleware)

    // Register protected services
    apiv1.RegisterUserServiceHTTPServer(protectedRoute, w.service)

    return w.engine.Start(ctx)
}
```

## Best Practices

### 1. Password Security

```go
import "golang.org/x/crypto/bcrypt"

// Always hash passwords
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// Verify passwords
err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
```

### 2. Token Expiration

```go
// Short-lived access tokens (1 hour)
accessTokenExpiry := 1 * time.Hour

// Long-lived refresh tokens (30 days)
refreshTokenExpiry := 30 * 24 * time.Hour
```

### 3. Secure Token Storage

- Store tokens in HTTP-only cookies (web)
- Use secure storage (iOS Keychain, Android Keystore) for mobile
- Never store tokens in localStorage (XSS vulnerable)

### 4. CORS Configuration

```go
w.engine.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"https://yourdomain.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
}))
```

## AI Agent Checklist

When implementing authentication:

- [ ] JWT authorizer configured with secret key
- [ ] Auth middleware created and applied to routes
- [ ] Public vs protected routes clearly separated
- [ ] User schema includes password field (hashed)
- [ ] UserPlatform schema created for multi-platform support
- [ ] Login/Register/Refresh endpoints implemented
- [ ] Password hashing using bcrypt
- [ ] Token expiration configured appropriately
- [ ] Claims extracted correctly in handlers
- [ ] RBAC roles defined if needed
- [ ] ACL permissions configured if needed
- [ ] Error handling for invalid tokens
- [ ] CORS configured for frontend domains
