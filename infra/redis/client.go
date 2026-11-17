package redis

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Config defines the configuration parameters for establishing a Redis connection.
type Config struct {
	// URL is parsed by go-redis ParseURL, for example
	// "redis://user:password@localhost:6379/0" or "rediss://..." for TLS. The
	// path selects the database number.
	URL string `json:"url" yaml:"url"`
}

// NewClient creates and returns a new Redis client instance based on the provided configuration.
// It only parses the Redis URL and builds the client; go-redis connects lazily on first use, so
// connectivity errors surface when the client is actually used rather than at construction time.
// Returns an error if the URL is invalid.
func NewClient(conf Config) (*redis.Client, error) {
	options, err := redis.ParseURL(conf.URL)
	if err != nil {
		// A *url.Error embeds the raw URL, credentials included; go-redis's
		// setupTCPConn can also reflect u.Path (and embedded user:pass@host)
		// for single-slash URLs like "redis:/user:pass@host/0". Redact both.
		if uErr, ok := errors.AsType[*url.Error](err); ok {
			return nil, fmt.Errorf("redis: invalid url: %w", uErr.Err)
		}
		if strings.Contains(err.Error(), "@") || containsParsedPassword(conf.URL, err.Error()) {
			return nil, errors.New("redis: invalid url")
		}
		return nil, err
	}
	return redis.NewClient(options), nil
}

func containsParsedPassword(rawURL, errMsg string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.User == nil {
		return false
	}
	pw, ok := u.User.Password()
	return ok && pw != "" && strings.Contains(errMsg, pw)
}
