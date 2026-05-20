package code

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	resetCodePrefix = "reset_code:"
	codeTTL         = 5 * time.Minute
)

var redisClient *redis.Client

// SetRedisClient set redis client for code storage
func SetRedisClient(client *redis.Client) {
	redisClient = client
}

// GenerateCode generate a 6-digit numeric code
func GenerateCode() string {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%06d", rng.Intn(1000000))
}

// SaveResetCode save reset code to redis
func SaveResetCode(ctx context.Context, target string, code string) error {
	if redisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	key := resetCodePrefix + target
	return redisClient.Set(ctx, key, code, codeTTL).Err()
}

// VerifyResetCode verify reset code from redis
func VerifyResetCode(ctx context.Context, target string, code string) (bool, error) {
	if redisClient == nil {
		return false, fmt.Errorf("redis client not initialized")
	}
	key := resetCodePrefix + target
	storedCode, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if storedCode != code {
		return false, nil
	}
	// delete code after successful verification
	_ = redisClient.Del(ctx, key).Err()
	return true, nil
}
