package redis

import "github.com/redis/go-redis/v9"

type Config struct {
	Addr     string `json:"addr"`
	DB       int    `json:"db"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewClient(conf *Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		DB:       conf.DB,
		Username: conf.Username,
		Password: conf.Password,
	})
}
