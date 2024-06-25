package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"net/http"
	"strconv"
)

type JwtAuth struct {
	Key    string
	fields JwtFields
}

func NewJwtAuth(key string) *JwtAuth {
	return &JwtAuth{
		Key:    key,
		fields: NewJwtFields(),
	}
}

type JwtFields struct {
	ID                  string `json:"id"`
	Username            string `json:"username"`
	Permission          string `json:"permission"`
	AuthorizationHeader string `json:"authorization_header"`
}

func NewJwtFields() JwtFields {
	return JwtFields{
		ID:                  "id",
		Username:            "username",
		Permission:          "permission",
		AuthorizationHeader: "Authorization",
	}
}

type jwtError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (j jwtError) Error() string {
	return j.Message
}

func (j jwtError) Status() int {
	return j.Code
}

var (
	NeedLoginError  = jwtError{401, "need login"}
	PermissionError = jwtError{403, "permission denied"}
)

func (w *JwtAuth) CreateJwtToken(id int, username string, permission ...string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		w.fields.ID:         strconv.Itoa(id),
		w.fields.Username:   username,
		w.fields.Permission: permission,
	})
	tokenString, err := token.SignedString([]byte(w.Key))
	return tokenString, err
}

func (w *JwtAuth) GetCurrentID(ctx *gin.Context) (int, error) {
	raw, exist := ctx.Get(w.fields.ID)
	if !exist {
		return 0, NeedLoginError
	}
	id, ok := raw.(int)
	if !ok {
		return 0, NeedLoginError
	}
	return id, nil
}

func (w *JwtAuth) CheckAuthStatus(ctx *gin.Context) error {
	_, err := w.GetCurrentID(ctx)
	return err
}

func (w *JwtAuth) CheckAuthID(ctx *gin.Context, id int) error {
	currentId, err := w.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}

func (w *JwtAuth) CheckAuthPermission(ctx *gin.Context, permission string) error {
	permissionList, exist := ctx.Get(w.fields.Permission)
	if !exist {
		return PermissionError
	}
	permissions := permissionList.([]any)
	for _, p := range permissions {
		if p == permission || p == "all" {
			return nil
		}
	}
	return PermissionError
}

func (w *JwtAuth) NewJwtAuthMiddleware(abortOnError bool) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(w.fields.AuthorizationHeader)
		abort := func() {
			if abortOnError {
				ctx.JSON(http.StatusUnauthorized, gin.H{
					"message": "unauthorized",
				})
				ctx.Abort()
			}
		}
		if token == "" {
			abort()
			return
		}
		t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(w.Key), nil
		})
		if err != nil || !t.Valid {
			abort()
			return
		}
		claims, valid := t.Claims.(jwt.MapClaims)
		if !valid {
			abort()
			return
		}
		if idRaw, ok := claims[w.fields.ID].(string); ok {
			if id, e := strconv.Atoi(idRaw); e == nil {
				ctx.Set("id", id)
			}
		}
		if username, ok := claims[w.fields.Username].(string); ok {
			ctx.Set("username", username)
		}
		if permission, ok := claims[w.fields.Permission].([]any); ok {
			ctx.Set("permission", permission)
		}
	}
}

func (w *JwtAuth) JwtAuthMiddleware(ctx *gin.Context) {
	w.NewJwtAuthMiddleware(true)(ctx)
}

func (w *JwtAuth) NewPermissionMiddleware(per string) func(context *gin.Context) {
	return func(context *gin.Context) {
		err := w.CheckAuthPermission(context, per)
		if err != nil {
			context.JSON(http.StatusForbidden, gin.H{
				"message": err.Error(),
			})
			context.Abort()
		}
	}
}
