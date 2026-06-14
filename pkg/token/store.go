package token

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func NewStore(redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connect: %w", err)
	}
	return &Store{client: client}, nil
}

func (s *Store) Client() *redis.Client {
	return s.client
}

func (s *Store) Save(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, hashKey(token), userID, ttl).Err()
}

func (s *Store) Exists(ctx context.Context, token string) (string, error) {
	return s.client.Get(ctx, hashKey(token)).Result()
}

func (s *Store) Revoke(ctx context.Context, token string) error {
	return s.client.Del(ctx, hashKey(token)).Err()
}

func (s *Store) SaveVerification(ctx context.Context, token, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, verifyKey(token), userID, ttl).Err()
}

func (s *Store) GetVerification(ctx context.Context, token string) (string, error) {
	return s.client.Get(ctx, verifyKey(token)).Result()
}

func (s *Store) RevokeVerification(ctx context.Context, token string) error {
	return s.client.Del(ctx, verifyKey(token)).Err()
}

func hashKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("refresh:%x", h)
}

func verifyKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("verify:%x", h)
}
