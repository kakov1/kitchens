package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"kitchens/internal/domain"
)

type OrderService interface {
	CreateOrder(ctx context.Context, input *domain.CreateOrderInput) (*domain.Order, error)
	GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error)
	GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error)
	UpdateStatus(ctx context.Context, orderID, restaurantID uuid.UUID, newStatus domain.OrderStatus) error
}

type OrderHandler struct {
	usecase OrderService
	lgr     *slog.Logger
}

func NewOrderHandler(usecase OrderService, lgr *slog.Logger) *OrderHandler {
	return &OrderHandler{
		usecase: usecase,
		lgr:     lgr,
	}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	const op = "OrderHandler.CreateOrder"

	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, op, err, h.lgr)
		return
	}

	var in domain.CreateOrderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}
	in.UserID = userID

	order, err := h.usecase.CreateOrder(r.Context(), &in)
	if err != nil {
		if errors.Is(err, domain.ErrMenuItemUnavailable) ||
			errors.Is(err, domain.ErrRestaurantMismatch) {
			respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
			return
		}

		respondWithError(w, http.StatusInternalServerError, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusCreated, order, h.lgr)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	const op = "OrderHandler.GetOrder"

	orderIDStr := r.PathValue("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}

	order, err := h.usecase.GetOrderByID(r.Context(), orderID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusOK, order, h.lgr)
}

func (h *OrderHandler) GetRestaurantOrders(w http.ResponseWriter, r *http.Request) {
	const op = "OrderHandler.GetRestaurantOrders"

	restaurantIDStr := r.Header.Get("X-Restaurant-ID")
	restaurantID, err := uuid.Parse(restaurantIDStr)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, op, err, h.lgr)
		return
	}

	var statusPtr *domain.OrderStatus
	if statusStr := r.URL.Query().Get("status_id"); statusStr != "" {
		statusInt, err := strconv.Atoi(statusStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
			return
		}
		status := domain.OrderStatus(statusInt)
		statusPtr = &status
	}

	orders, err := h.usecase.GetOrdersByRestaurant(r.Context(), restaurantID, statusPtr)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusOK, orders, h.lgr)
}

func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	const op = "OrderHandler.UpdateOrderStatus"

	restaurantIDStr := r.Header.Get("X-Restaurant-ID")
	restaurantID, err := uuid.Parse(restaurantIDStr)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, op, err, h.lgr)
		return
	}

	orderIDStr := r.PathValue("id")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}

	var req struct {
		StatusID int `json:"status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}

	err = h.usecase.UpdateStatus(r.Context(), orderID, restaurantID, domain.OrderStatus(req.StatusID))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"}, h.lgr)
}
