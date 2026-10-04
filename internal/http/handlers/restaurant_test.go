package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"kitchens/internal/domain"
	"kitchens/internal/http/handlers"
)

type mockRestaurantHandlerService struct {
	mock.Mock
}

func (m *mockRestaurantHandlerService) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	args := m.Called(ctx)
	if res := args.Get(0); res != nil {
		return res.([]domain.Restaurant), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockRestaurantHandlerService) GetMenu(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error) {
	args := m.Called(ctx, restaurantID)
	if res := args.Get(0); res != nil {
		return res.([]domain.MenuItem), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestRestaurantHandler_GetRestaurants(t *testing.T) {
	lgr := getDiscardLogger()

	t.Run("success", func(t *testing.T) {
		svc := new(mockRestaurantHandlerService)
		h := handlers.NewRestaurantHandler(svc, lgr)

		expected := []domain.Restaurant{
			{ID: uuid.New(), Name: "Rest 1", Description: "Desc 1"},
		}
		svc.On("GetRestaurants", mock.Anything).Return(expected, nil)

		req := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
		rec := httptest.NewRecorder()

		h.GetRestaurants(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("internal server error", func(t *testing.T) {
		svc := new(mockRestaurantHandlerService)
		h := handlers.NewRestaurantHandler(svc, lgr)

		svc.On("GetRestaurants", mock.Anything).Return(nil, errors.New("db failure"))

		req := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
		rec := httptest.NewRecorder()

		h.GetRestaurants(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		svc.AssertExpectations(t)
	})
}

func TestRestaurantHandler_GetMenu(t *testing.T) {
	lgr := getDiscardLogger()
	restID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc := new(mockRestaurantHandlerService)
		h := handlers.NewRestaurantHandler(svc, lgr)

		expected := []domain.MenuItem{
			{ID: uuid.New(), RestaurantID: restID, Name: "Burger", Price: 350, IsAvailable: true},
		}
		svc.On("GetMenu", mock.Anything, restID).Return(expected, nil)

		req := httptest.NewRequest(http.MethodGet, "/restaurants/"+restID.String()+"/menu", nil)
		req.SetPathValue("id", restID.String())
		rec := httptest.NewRecorder()

		h.GetMenu(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		svc.AssertExpectations(t)
	})

	t.Run("invalid restaurant id in path", func(t *testing.T) {
		svc := new(mockRestaurantHandlerService)
		h := handlers.NewRestaurantHandler(svc, lgr)

		req := httptest.NewRequest(http.MethodGet, "/restaurants/invalid-uuid/menu", nil)
		req.SetPathValue("id", "invalid-uuid")
		rec := httptest.NewRecorder()

		h.GetMenu(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := new(mockRestaurantHandlerService)
		h := handlers.NewRestaurantHandler(svc, lgr)

		svc.On("GetMenu", mock.Anything, restID).Return(nil, errors.New("service failure"))

		req := httptest.NewRequest(http.MethodGet, "/restaurants/"+restID.String()+"/menu", nil)
		req.SetPathValue("id", restID.String())
		rec := httptest.NewRecorder()

		h.GetMenu(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		svc.AssertExpectations(t)
	})
}
