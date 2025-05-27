package dash

import (
	"encoding/json"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/server/middleware/auth"
	"github.com/gin-gonic/gin"
)

func NewPureAdminCookieAuthMiddleware[T authorizer.UID](authParser authorizer.Parser[authorizer.RBACClaims[T]]) gin.HandlerFunc {
	return auth.NewCookieAuthMiddleware("authorized-token", func(raw string) (string, error) {
		var token struct {
			AccessToken string `json:"accessToken"`
		}
		err := json.Unmarshal([]byte(raw), &token)
		if err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}, authParser, true)
}
