package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"kitchens/internal/domain"
	"kitchens/internal/http/handlers"
)

type mockOrderService struct {
	mock.Mock
}

func (m *mockOrderService) CreateOrder(ctx context.Context, input *domain.CreateOrderInput) (*domain.Order, error) {
	args := m.Called(ctx, input)
	if res := args.Get(0); res != nil {
		return res.(*domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderService) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	args := m.Called(ctx, orderID)
	if res := args.Get(0); res != nil {
		return res.(*domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderService) GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error) {
	args := m.Called(ctx, restaurantID, statusID)
	if res := args.Get(0); res != nil {
		return res.([]domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderService) UpdateStatus(ctx context.Context, orderID, restaurantID uuid.UUID, newStatus domain.OrderStatus) error {
	args := m.Called(ctx, orderID, restaurantID, newStatus)
	return args.Error(0)
}

func getDiscardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	lgr := getDiscardLogger()
	userID := uuid.New()
	restID := uuid.New()
	menuItemID := uuid.New()

	validBody := func() []byte {
		body, _ := json.Marshal(domain.CreateOrderInput{
			RestaurantID: restID,
			Items: []domain.CreateOrderItemInput{
				{MenuItemID: menuItemID, Quantity: 2},
			},
		})
		return body
	}

	t.Run("success", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("CreateOrder", mock.Anything, mock.MatchedBy(func(in *domain.CreateOrderInput) bool {
			return in.UserID == userID && in.RestaurantID == restID
		})).Return(&domain.Order{ID: uuid.New(), UserID: userID, RestaurantID: restID}, nil)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validBody()))
		req.Header.Set("X-User-ID", userID.String())
		rec := httptest.NewRecorder()

		h.CreateOrder(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("missing or invalid X-User-ID header", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validBody()))
		req.Header.Set("X-User-ID", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.CreateOrder(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid json body", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("X-User-ID", userID.String())
		rec := httptest.NewRecorder()

		h.CreateOrder(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("domain validation error", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("CreateOrder", mock.Anything, mock.Anything).Return(nil, domain.ErrMenuItemUnavailable)

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validBody()))
		req.Header.Set("X-User-ID", userID.String())
		rec := httptest.NewRecorder()

		h.CreateOrder(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("internal server error", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("CreateOrder", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

		req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(validBody()))
		req.Header.Set("X-User-ID", userID.String())
		rec := httptest.NewRecorder()

		h.CreateOrder(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestOrderHandler_GetOrder(t *testing.T) {
	lgr := getDiscardLogger()
	orderID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("GetOrderByID", mock.Anything, orderID).Return(&domain.Order{ID: orderID}, nil)

		req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.GetOrder(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("invalid path id", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodGet, "/orders/bad-id", nil)
		req.SetPathValue("id", "bad-id")
		rec := httptest.NewRecorder()

		h.GetOrder(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("order not found", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("GetOrderByID", mock.Anything, orderID).Return(nil, domain.ErrNotFound)

		req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.GetOrder(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestOrderHandler_GetRestaurantOrders(t *testing.T) {
	lgr := getDiscardLogger()
	restID := uuid.New()

	t.Run("success with status filter", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		expectedStatus := domain.OrderStatus(2)
		svc.On("GetOrdersByRestaurant", mock.Anything, restID, &expectedStatus).Return([]domain.Order{{ID: uuid.New()}}, nil)

		req := httptest.NewRequest(http.MethodGet, "/restaurant/orders?status_id=2", nil)
		req.Header.Set("X-Restaurant-ID", restID.String())
		rec := httptest.NewRecorder()

		h.GetRestaurantOrders(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("missing or invalid X-Restaurant-ID header", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodGet, "/restaurant/orders", nil)
		rec := httptest.NewRecorder()

		h.GetRestaurantOrders(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid status_id query parameter", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodGet, "/restaurant/orders?status_id=abc", nil)
		req.Header.Set("X-Restaurant-ID", restID.String())
		rec := httptest.NewRecorder()

		h.GetRestaurantOrders(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("GetOrdersByRestaurant", mock.Anything, restID, (*domain.OrderStatus)(nil)).Return(nil, errors.New("db error"))

		req := httptest.NewRequest(http.MethodGet, "/restaurant/orders", nil)
		req.Header.Set("X-Restaurant-ID", restID.String())
		rec := httptest.NewRecorder()

		h.GetRestaurantOrders(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestOrderHandler_UpdateOrderStatus(t *testing.T) {
	lgr := getDiscardLogger()
	restID := uuid.New()
	orderID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("UpdateStatus", mock.Anything, orderID, restID, domain.OrderStatus(2)).Return(nil)

		body := []byte(`{"status_id": 2}`)
		req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status", bytes.NewReader(body))
		req.Header.Set("X-Restaurant-ID", restID.String())
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.UpdateOrderStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("unauthorized restaurant header", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status", bytes.NewReader([]byte(`{"status_id": 2}`)))
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.UpdateOrderStatus(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid path id", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodPatch, "/orders/invalid/status", bytes.NewReader([]byte(`{"status_id": 2}`)))
		req.Header.Set("X-Restaurant-ID", restID.String())
		req.SetPathValue("id", "invalid")
		rec := httptest.NewRecorder()

		h.UpdateOrderStatus(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body json", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status", bytes.NewReader([]byte(`{bad json`)))
		req.Header.Set("X-Restaurant-ID", restID.String())
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.UpdateOrderStatus(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service transition error", func(t *testing.T) {
		svc := new(mockOrderService)
		h := handlers.NewOrderHandler(svc, lgr)

		svc.On("UpdateStatus", mock.Anything, orderID, restID, domain.OrderStatus(5)).Return(domain.ErrInvalidStatusChange)

		req := httptest.NewRequest(http.MethodPatch, "/orders/"+orderID.String()+"/status", bytes.NewReader([]byte(`{"status_id": 5}`)))
		req.Header.Set("X-Restaurant-ID", restID.String())
		req.SetPathValue("id", orderID.String())
		rec := httptest.NewRecorder()

		h.UpdateOrderStatus(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
