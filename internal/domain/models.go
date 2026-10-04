package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrRestaurantNotFound  = errors.New("restaurant not found")
	ErrEmptyOrderItems     = errors.New("order must contain at least one item")
	ErrInvalidQuantity     = errors.New("item quantity must be greater than zero")
	ErrMenuItemUnavailable = errors.New("menu item is unavailable")
	ErrRestaurantMismatch  = errors.New("menu item does not belong to the specified restaurant")
	ErrInvalidStatusChange = errors.New("invalid status transition")
)

type Restaurant struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type MenuItem struct {
	ID           uuid.UUID `json:"id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	Name         string    `json:"name"`
	Price        int       `json:"price"` // в копейках
	IsAvailable  bool      `json:"is_available"`
}

type OrderStatus int

const (
	StatusCreated   OrderStatus = 1
	StatusAccepted  OrderStatus = 2
	StatusCooking   OrderStatus = 3
	StatusReady     OrderStatus = 4
	StatusCancelled OrderStatus = 5
)

type OrderItem struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	MenuItemID      uuid.UUID `json:"menu_item_id"`
	Quantity        int       `json:"quantity"`
	PriceAtPurchase int       `json:"price_at_purchase"`
}

type Order struct {
	ID           uuid.UUID   `json:"id"`
	UserID       uuid.UUID   `json:"user_id"`
	RestaurantID uuid.UUID   `json:"restaurant_id"`
	StatusID     OrderStatus `json:"status_id"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Items        []OrderItem `json:"items,omitempty"`
}

type CreateOrderItemInput struct {
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Quantity   int       `json:"quantity"`
}

type CreateOrderInput struct {
	UserID       uuid.UUID              `json:"user_id"`
	RestaurantID uuid.UUID              `json:"restaurant_id"`
	Items        []CreateOrderItemInput `json:"items"`
}

func (in *CreateOrderInput) Validate() error {
	if in.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}

	if in.RestaurantID == uuid.Nil {
		return errors.New("restaurant_id is required")
	}

	if len(in.Items) == 0 {
		return ErrEmptyOrderItems
	}

	for _, item := range in.Items {
		if item.MenuItemID == uuid.Nil {
			return errors.New("menu_item_id is required")
		}
		if item.Quantity <= 0 {
			return ErrInvalidQuantity
		}
	}
	return nil
}
