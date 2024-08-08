package telegrambotgo

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
)

type Redis struct {
	Host     string
	Port     uint32
	Password string
	DB       int
}

func (r *Redis) RedisClient() *redis.Client {
	if redisClient == nil {
		redisClient = redis.NewClient(
			&redis.Options{
				Addr:     fmt.Sprintf("%s:%d", r.Host, r.Port),
				Password: r.Password,
				DB:       r.DB,
			},
		)
		_, err := redisClient.Ping(ctx).Result()
		if err != nil {
			fmt.Println("**** ERROR **** REDIS IS NOT AVAILABLE")
			panic(err)
		}
	}

	return redisClient
}
