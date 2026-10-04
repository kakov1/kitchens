package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"kitchens/internal/domain"
)

type RestaurantProvider interface {
	GetRestaurants(ctx context.Context) ([]domain.Restaurant, error)
	GetMenu(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error)
}

type MenuCacheProvider interface {
	Get(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error)
	Set(ctx context.Context, restaurantID uuid.UUID, items []domain.MenuItem) error
	Delete(ctx context.Context, restaurantID uuid.UUID) error
}

type RestaurantService struct {
	repo  RestaurantProvider
	cache MenuCacheProvider
	lgr   *slog.Logger
}

func NewRestaurantService(repo RestaurantProvider, cache MenuCacheProvider, lgr *slog.Logger) *RestaurantService {
	return &RestaurantService{
		repo:  repo,
		cache: cache,
		lgr:   lgr,
	}
}

func (s *RestaurantService) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	return s.repo.GetRestaurants(ctx)
}

func (s *RestaurantService) GetMenu(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error) {
	if restaurantID == uuid.Nil {
		return nil, errors.New("invalid restaurant id")
	}

	if s.cache != nil {
		cachedMenu, err := s.cache.Get(ctx, restaurantID)
		if err != nil && s.lgr != nil {
			s.lgr.Warn("cache lookup failed, falling back to db", slog.String("error", err.Error()))
		} else if cachedMenu != nil {
			return cachedMenu, nil
		}
	}

	menu, err := s.repo.GetMenu(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil && len(menu) > 0 {
		if err := s.cache.Set(ctx, restaurantID, menu); err != nil && s.lgr != nil {
			s.lgr.Warn("failed to write menu to cache", slog.String("error", err.Error()))
		}
	}

	return menu, nil
}
