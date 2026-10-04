package service

import (
	"context"
	"errors"
	"log/slog"

	"time"

	"github.com/google/uuid"

	"kitchens/internal/domain"
)

type OrderRepository interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
	GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, statusID domain.OrderStatus) error
}

type RestaurantRepository interface {
	GetMenuItemsByIDs(ctx context.Context, itemIDs []uuid.UUID) ([]domain.MenuItem, error)
}

type OrderEventProducer interface {
	PublishOrderCreated(ctx context.Context, orderID, restaurantID uuid.UUID) error
}

type OrderService struct {
	orderRepo OrderRepository
	restRepo  RestaurantRepository
	producer  OrderEventProducer
	lgr       *slog.Logger
}

func NewOrderService(orderRepo OrderRepository, restRepo RestaurantRepository, producer OrderEventProducer, lgr *slog.Logger) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		restRepo:  restRepo,
		producer:  producer,
		lgr:       lgr,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, input *domain.CreateOrderInput) (*domain.Order, error) {
	if input == nil {
		return nil, errors.New("input cannot be nil")
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	itemIDs := make([]uuid.UUID, 0, len(input.Items))
	for _, item := range input.Items {
		itemIDs = append(itemIDs, item.MenuItemID)
	}

	menuItems, err := s.restRepo.GetMenuItemsByIDs(ctx, itemIDs)
	if err != nil {
		s.lgr.Error("failed to get menu items", slog.String("error", err.Error()))
		return nil, err
	}

	menuItemsMap := make(map[uuid.UUID]domain.MenuItem)
	for _, mi := range menuItems {
		menuItemsMap[mi.ID] = mi
	}

	order := &domain.Order{
		ID:           uuid.New(),
		UserID:       input.UserID,
		RestaurantID: input.RestaurantID,
		StatusID:     domain.StatusCreated,
	}

	for _, itemInput := range input.Items {
		mi, exists := menuItemsMap[itemInput.MenuItemID]
		if !exists || !mi.IsAvailable {
			return nil, domain.ErrMenuItemUnavailable
		}

		if mi.RestaurantID != input.RestaurantID {
			return nil, domain.ErrRestaurantMismatch
		}

		order.Items = append(order.Items, domain.OrderItem{
			ID:              uuid.New(),
			OrderID:         order.ID,
			MenuItemID:      mi.ID,
			Quantity:        itemInput.Quantity,
			PriceAtPurchase: mi.Price,
		})
	}

	if err := s.orderRepo.CreateOrder(ctx, order); err != nil {
		s.lgr.Error("failed to save order", slog.String("error", err.Error()))
		return nil, err
	}

	s.lgr.Info("order created successfully", slog.String("order_id", order.ID.String()))

	if s.producer != nil {
		go func(orderID, restID uuid.UUID) {
			pubCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.producer.PublishOrderCreated(pubCtx, orderID, restID)
		}(order.ID, order.RestaurantID)
	}

	return order, nil
}

func (s *OrderService) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		s.lgr.Error("failed to get order by id", slog.String("error", err.Error()))
		return nil, err
	}

	return order, nil
}

func (s *OrderService) GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error) {
	orders, err := s.orderRepo.GetOrdersByRestaurant(ctx, restaurantID, statusID)
	if err != nil {
		s.lgr.Error("failed to get restaurant orders", slog.String("error", err.Error()))
		return nil, err
	}

	return orders, nil
}

func (s *OrderService) UpdateStatus(ctx context.Context, orderID, restaurantID uuid.UUID, newStatus domain.OrderStatus) error {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}
		s.lgr.Error("failed to get order", slog.String("error", err.Error()))
		return err
	}

	if order.RestaurantID != restaurantID {
		return errors.New("forbidden: order belongs to another restaurant")
	}

	if !isValidStatusTransition(order.StatusID, newStatus) {
		return domain.ErrInvalidStatusChange
	}

	if err := s.orderRepo.UpdateOrderStatus(ctx, orderID, newStatus); err != nil {
		s.lgr.Error("failed to update order status", slog.String("error", err.Error()))
		return err
	}

	s.lgr.Info("order status updated", slog.String("order_id", orderID.String()), slog.Int("new_status", int(newStatus)))
	return nil
}

func isValidStatusTransition(oldStatus, newStatus domain.OrderStatus) bool {
	if newStatus == domain.StatusCancelled && oldStatus != domain.StatusReady {
		return true
	}
	return newStatus == oldStatus+1
}
