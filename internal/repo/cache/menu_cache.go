package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"kitchens/internal/domain"
)

const menuTTL = 5 * time.Minute

type MenuCache struct {
	client *redis.Client
}

func NewMenuCache(client *redis.Client) *MenuCache {
	return &MenuCache{client: client}
}

func (c *MenuCache) Get(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error) {
	key := fmt.Sprintf("menu:%s", restaurantID.String())

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Cache miss — не ошибка, идем в БД
		}
		return nil, fmt.Errorf("redis get menu: %w", err)
	}

	var items []domain.MenuItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("unmarshal cached menu: %w", err)
	}

	return items, nil
}

func (c *MenuCache) Set(ctx context.Context, restaurantID uuid.UUID, items []domain.MenuItem) error {
	key := fmt.Sprintf("menu:%s", restaurantID.String())

	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal menu for cache: %w", err)
	}

	if err := c.client.Set(ctx, key, data, menuTTL).Err(); err != nil {
		return fmt.Errorf("redis set menu: %w", err)
	}

	return nil
}

func (c *MenuCache) Delete(ctx context.Context, restaurantID uuid.UUID) error {
	key := fmt.Sprintf("menu:%s", restaurantID.String())
	return c.client.Del(ctx, key).Err()
}
