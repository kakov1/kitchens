package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"kitchens/internal/domain"
	"kitchens/internal/service"
)

type mockOrderRepo struct {
	mock.Mock
}

func (m *mockOrderRepo) CreateOrder(ctx context.Context, order *domain.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *mockOrderRepo) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	args := m.Called(ctx, orderID)
	if res := args.Get(0); res != nil {
		return res.(*domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderRepo) GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error) {
	args := m.Called(ctx, restaurantID, statusID)
	if res := args.Get(0); res != nil {
		return res.([]domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderRepo) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, statusID domain.OrderStatus) error {
	args := m.Called(ctx, orderID, statusID)
	return args.Error(0)
}

type mockRestaurantRepo struct {
	mock.Mock
}

func (m *mockRestaurantRepo) GetMenuItemsByIDs(ctx context.Context, itemIDs []uuid.UUID) ([]domain.MenuItem, error) {
	args := m.Called(ctx, itemIDs)
	if res := args.Get(0); res != nil {
		return res.([]domain.MenuItem), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestOrderService_CreateOrder(t *testing.T) {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	userID := uuid.New()
	restaurantID := uuid.New()
	menuItemID := uuid.New()

	t.Run("success", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		input := &domain.CreateOrderInput{
			UserID:       userID,
			RestaurantID: restaurantID,
			Items: []domain.CreateOrderItemInput{
				{MenuItemID: menuItemID, Quantity: 2},
			},
		}

		restRepo.On("GetMenuItemsByIDs", mock.Anything, []uuid.UUID{menuItemID}).
			Return([]domain.MenuItem{
				{ID: menuItemID, RestaurantID: restaurantID, Name: "Pizza", Price: 50000, IsAvailable: true},
			}, nil)

		orderRepo.On("CreateOrder", mock.Anything, mock.AnythingOfType("*domain.Order")).
			Return(nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.CreateOrder(context.Background(), input)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.StatusCreated, res.StatusID)
		orderRepo.AssertExpectations(t)
		restRepo.AssertExpectations(t)
	})

	t.Run("menu_item_unavailable", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		input := &domain.CreateOrderInput{
			UserID:       userID,
			RestaurantID: restaurantID,
			Items: []domain.CreateOrderItemInput{
				{MenuItemID: menuItemID, Quantity: 1},
			},
		}

		restRepo.On("GetMenuItemsByIDs", mock.Anything, []uuid.UUID{menuItemID}).
			Return([]domain.MenuItem{
				{ID: menuItemID, RestaurantID: restaurantID, Name: "Pizza", Price: 50000, IsAvailable: false},
			}, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.CreateOrder(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrMenuItemUnavailable)
		assert.Nil(t, res)
	})

	t.Run("restaurant_mismatch", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)
		otherRestaurantID := uuid.New()

		input := &domain.CreateOrderInput{
			UserID:       userID,
			RestaurantID: restaurantID,
			Items: []domain.CreateOrderItemInput{
				{MenuItemID: menuItemID, Quantity: 1},
			},
		}

		restRepo.On("GetMenuItemsByIDs", mock.Anything, []uuid.UUID{menuItemID}).
			Return([]domain.MenuItem{
				{ID: menuItemID, RestaurantID: otherRestaurantID, Name: "Pizza", Price: 50000, IsAvailable: true},
			}, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.CreateOrder(context.Background(), input)

		assert.ErrorIs(t, err, domain.ErrRestaurantMismatch)
		assert.Nil(t, res)
	})
}

func TestOrderService_UpdateStatus(t *testing.T) {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	orderID := uuid.New()
	restaurantID := uuid.New()
	otherRestaurantID := uuid.New()

	t.Run("success_valid_transition", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&domain.Order{
			ID:           orderID,
			RestaurantID: restaurantID,
			StatusID:     domain.StatusCreated,
		}, nil)

		orderRepo.On("UpdateOrderStatus", mock.Anything, orderID, domain.StatusAccepted).Return(nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		err := svc.UpdateStatus(context.Background(), orderID, restaurantID, domain.StatusAccepted)

		assert.NoError(t, err)
		orderRepo.AssertExpectations(t)
	})

	t.Run("forbidden_restaurant_mismatch", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&domain.Order{
			ID:           orderID,
			RestaurantID: restaurantID,
			StatusID:     domain.StatusCreated,
		}, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		err := svc.UpdateStatus(context.Background(), orderID, otherRestaurantID, domain.StatusAccepted)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("invalid_status_transition", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&domain.Order{
			ID:           orderID,
			RestaurantID: restaurantID,
			StatusID:     domain.StatusCreated,
		}, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)

		err := svc.UpdateStatus(context.Background(), orderID, restaurantID, domain.StatusReady)

		assert.ErrorIs(t, err, domain.ErrInvalidStatusChange)
	})
}

func TestOrderService_GetOrderByID(t *testing.T) {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	orderID := uuid.New()

	t.Run("success", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)
		expected := &domain.Order{ID: orderID, StatusID: domain.StatusCreated}

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(expected, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.GetOrderByID(context.Background(), orderID)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
	})

	t.Run("not_found", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(nil, domain.ErrNotFound)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.GetOrderByID(context.Background(), orderID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Nil(t, res)
	})

	t.Run("repo_error", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(nil, errors.New("db error"))

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.GetOrderByID(context.Background(), orderID)

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestOrderService_GetOrdersByRestaurant(t *testing.T) {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	restID := uuid.New()

	t.Run("success", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)
		expected := []domain.Order{
			{ID: uuid.New(), RestaurantID: restID, StatusID: domain.StatusCreated, CreatedAt: time.Now()},
		}

		orderRepo.On("GetOrdersByRestaurant", mock.Anything, restID, (*domain.OrderStatus)(nil)).Return(expected, nil)

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.GetOrdersByRestaurant(context.Background(), restID, nil)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
	})

	t.Run("repo_error", func(t *testing.T) {
		orderRepo := new(mockOrderRepo)
		restRepo := new(mockRestaurantRepo)

		orderRepo.On("GetOrdersByRestaurant", mock.Anything, restID, (*domain.OrderStatus)(nil)).Return(nil, errors.New("db error"))

		svc := service.NewOrderService(orderRepo, restRepo, nil, lgr)
		res, err := svc.GetOrdersByRestaurant(context.Background(), restID, nil)

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}
