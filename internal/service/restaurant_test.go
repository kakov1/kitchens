package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"kitchens/internal/domain"
	"kitchens/internal/service"
)

type mockLocalRestaurantRepo struct {
	mock.Mock
}

func (m *mockLocalRestaurantRepo) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	args := m.Called(ctx)
	if res := args.Get(0); res != nil {
		return res.([]domain.Restaurant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLocalRestaurantRepo) GetMenu(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	args := m.Called(ctx, restID)
	if res := args.Get(0); res != nil {
		return res.([]domain.MenuItem), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockLocalMenuCache struct {
	mock.Mock
}

func (m *mockLocalMenuCache) Get(ctx context.Context, restID uuid.UUID) ([]domain.MenuItem, error) {
	args := m.Called(ctx, restID)
	if res := args.Get(0); res != nil {
		return res.([]domain.MenuItem), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLocalMenuCache) Set(ctx context.Context, restID uuid.UUID, items []domain.MenuItem) error {
	args := m.Called(ctx, restID, items)
	return args.Error(0)
}

func (m *mockLocalMenuCache) Delete(ctx context.Context, restID uuid.UUID) error {
	args := m.Called(ctx, restID)
	return args.Error(0)
}

func TestRestaurantService_GetRestaurants(t *testing.T) {
	ctx := context.Background()
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	t.Run("success", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)
		expected := []domain.Restaurant{{ID: uuid.New(), Name: "Rest 1"}}

		repo.On("GetRestaurants", ctx).Return(expected, nil)

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetRestaurants(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		repo.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)

		repo.On("GetRestaurants", ctx).Return(nil, errors.New("db error"))

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetRestaurants(ctx)

		assert.Error(t, err)
		assert.Nil(t, res)
		repo.AssertExpectations(t)
	})
}

func TestRestaurantService_GetMenu_CacheAside(t *testing.T) {
	ctx := context.Background()
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	restID := uuid.New()
	menuItems := []domain.MenuItem{
		{ID: uuid.New(), RestaurantID: restID, Name: "Burger", Price: 35000, IsAvailable: true},
	}

	t.Run("cache hit - db is not called", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)

		cache.On("Get", ctx, restID).Return(menuItems, nil)

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetMenu(ctx, restID)

		assert.NoError(t, err)
		assert.Equal(t, menuItems, res)
		repo.AssertNotCalled(t, "GetMenu", mock.Anything, mock.Anything)
		cache.AssertExpectations(t)
	})

	t.Run("cache miss - db is called and cache is populated", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)

		cache.On("Get", ctx, restID).Return(nil, nil)
		repo.On("GetMenu", ctx, restID).Return(menuItems, nil)
		cache.On("Set", ctx, restID, menuItems).Return(nil)

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetMenu(ctx, restID)

		assert.NoError(t, err)
		assert.Equal(t, menuItems, res)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("cache failure - graceful fallback to db", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)

		cache.On("Get", ctx, restID).Return(nil, errors.New("redis timeout"))
		repo.On("GetMenu", ctx, restID).Return(menuItems, nil)
		cache.On("Set", ctx, restID, menuItems).Return(nil)

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetMenu(ctx, restID)

		assert.NoError(t, err)
		assert.Equal(t, menuItems, res)
		repo.AssertExpectations(t)
	})

	t.Run("invalid restaurant id", func(t *testing.T) {
		repo := new(mockLocalRestaurantRepo)
		cache := new(mockLocalMenuCache)

		svc := service.NewRestaurantService(repo, cache, lgr)
		res, err := svc.GetMenu(ctx, uuid.Nil)

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}
