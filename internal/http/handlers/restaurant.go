package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"kitchens/internal/domain"
)

type RestaurantService interface {
	GetRestaurants(ctx context.Context) ([]domain.Restaurant, error)
	GetMenu(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error)
}

type RestaurantHandler struct {
	usecase RestaurantService
	lgr     *slog.Logger
}

func NewRestaurantHandler(usecase RestaurantService, lgr *slog.Logger) *RestaurantHandler {
	return &RestaurantHandler{
		usecase: usecase,
		lgr:     lgr,
	}
}

func (h *RestaurantHandler) GetRestaurants(w http.ResponseWriter, r *http.Request) {
	const op = "RestaurantHandler.GetRestaurants"

	restaurants, err := h.usecase.GetRestaurants(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusOK, restaurants, h.lgr)
}

func (h *RestaurantHandler) GetMenu(w http.ResponseWriter, r *http.Request) {
	const op = "RestaurantHandler.GetMenu"

	restaurantIDStr := r.PathValue("id")
	restaurantID, err := uuid.Parse(restaurantIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, op, err, h.lgr)
		return
	}

	menu, err := h.usecase.GetMenu(r.Context(), restaurantID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, op, err, h.lgr)
		return
	}

	respondWithJSON(w, http.StatusOK, menu, h.lgr)
}
