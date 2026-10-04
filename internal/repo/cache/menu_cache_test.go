package cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"kitchens/internal/domain"
	"kitchens/internal/repo/cache"
)

func TestMenuCache_Get(t *testing.T) {
	ctx := context.Background()
	restID := uuid.New()
	key := "menu:" + restID.String()

	items := []domain.MenuItem{
		{
			ID:           uuid.New(),
			RestaurantID: restID,
			Name:         "Pizza Margherita",
			Price:        50000,
			IsAvailable:  true,
		},
	}
	data, _ := json.Marshal(items)

	t.Run("cache hit", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		c := cache.NewMenuCache(db)

		mock.ExpectGet(key).SetVal(string(data))

		res, err := c.Get(ctx, restID)
		assert.NoError(t, err)
		assert.Equal(t, items, res)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cache miss (redis.Nil)", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		c := cache.NewMenuCache(db)

		// SetErr вместо SetError
		mock.ExpectGet(key).SetErr(redis.Nil)

		res, err := c.Get(ctx, restID)
		assert.NoError(t, err)
		assert.Nil(t, res)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("redis failure", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		c := cache.NewMenuCache(db)

		// SetErr вместо SetError
		mock.ExpectGet(key).SetErr(assert.AnError)

		res, err := c.Get(ctx, restID)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMenuCache_SetAndDel(t *testing.T) {
	ctx := context.Background()
	restID := uuid.New()
	key := "menu:" + restID.String()

	items := []domain.MenuItem{
		{ID: uuid.New(), RestaurantID: restID, Name: "Pasta", Price: 40000, IsAvailable: true},
	}
	data, _ := json.Marshal(items)

	t.Run("set success", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		c := cache.NewMenuCache(db)

		mock.ExpectSet(key, data, 5*time.Minute).SetVal("OK")

		err := c.Set(ctx, restID, items)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("delete success", func(t *testing.T) {
		db, mock := redismock.NewClientMock()
		c := cache.NewMenuCache(db)

		mock.ExpectDel(key).SetVal(1)

		err := c.Delete(ctx, restID)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
