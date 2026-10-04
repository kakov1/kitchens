package kitchen_test

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"kitchens/internal/domain"
	kitchengrpc "kitchens/internal/grpc/kitchen"
	kitchenv1 "kitchens/pkg/api/kitchen/v1"
)

type mockOrderService struct {
	mock.Mock
}

func (m *mockOrderService) GetOrdersByRestaurant(ctx context.Context, restID uuid.UUID, st *domain.OrderStatus) ([]domain.Order, error) {
	args := m.Called(ctx, restID, st)
	if res := args.Get(0); res != nil {
		return res.([]domain.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrderService) UpdateStatus(ctx context.Context, orderID, restID uuid.UUID, st domain.OrderStatus) error {
	args := m.Called(ctx, orderID, restID, st)
	return args.Error(0)
}

func TestServer_GetOrders(t *testing.T) {
	ctx := context.Background()
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	restID := uuid.New()
	orderID := uuid.New()
	itemID := uuid.New()

	orders := []domain.Order{
		{
			ID:           orderID,
			UserID:       uuid.New(),
			RestaurantID: restID,
			StatusID:     domain.StatusCreated,
			CreatedAt:    time.Now(),
			Items: []domain.OrderItem{
				{ID: uuid.New(), MenuItemID: itemID, Quantity: 2, PriceAtPurchase: 55000},
			},
		},
	}

	t.Run("success without filter", func(t *testing.T) {
		svcMock := new(mockOrderService)
		srv := kitchengrpc.NewServer(svcMock, lgr)

		svcMock.On("GetOrdersByRestaurant", ctx, restID, (*domain.OrderStatus)(nil)).Return(orders, nil)

		resp, err := srv.GetOrders(ctx, &kitchenv1.GetOrdersRequest{
			RestaurantId: restID.String(),
			StatusId:     0,
		})

		assert.NoError(t, err)
		assert.Len(t, resp.GetOrders(), 1)
		assert.Equal(t, orderID.String(), resp.GetOrders()[0].GetId())
		assert.Equal(t, int32(domain.StatusCreated), resp.GetOrders()[0].GetStatusId())
		svcMock.AssertExpectations(t)
	})

	t.Run("invalid restaurant_id UUID", func(t *testing.T) {
		svcMock := new(mockOrderService)
		srv := kitchengrpc.NewServer(svcMock, lgr)

		_, err := srv.GetOrders(ctx, &kitchenv1.GetOrdersRequest{
			RestaurantId: "invalid-uuid",
		})

		assert.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("service internal error", func(t *testing.T) {
		svcMock := new(mockOrderService)
		srv := kitchengrpc.NewServer(svcMock, lgr)

		svcMock.On("GetOrdersByRestaurant", ctx, restID, (*domain.OrderStatus)(nil)).Return(nil, errors.New("db error"))

		_, err := srv.GetOrders(ctx, &kitchenv1.GetOrdersRequest{
			RestaurantId: restID.String(),
		})

		assert.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestServer_UpdateOrderStatus(t *testing.T) {
	ctx := context.Background()
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	restID := uuid.New()
	orderID := uuid.New()

	tests := []struct {
		name         string
		req          *kitchenv1.UpdateOrderStatusRequest
		setupMock    func(m *mockOrderService)
		expectedCode codes.Code
	}{
		{
			name: "success",
			req: &kitchenv1.UpdateOrderStatusRequest{
				OrderId:      orderID.String(),
				RestaurantId: restID.String(),
				StatusId:     2,
			},
			setupMock: func(m *mockOrderService) {
				m.On("UpdateStatus", ctx, orderID, restID, domain.OrderStatus(2)).Return(nil)
			},
			expectedCode: codes.OK,
		},
		{
			name: "invalid order_id UUID",
			req: &kitchenv1.UpdateOrderStatusRequest{
				OrderId:      "bad-id",
				RestaurantId: restID.String(),
				StatusId:     2,
			},
			setupMock:    func(m *mockOrderService) {},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "order not found",
			req: &kitchenv1.UpdateOrderStatusRequest{
				OrderId:      orderID.String(),
				RestaurantId: restID.String(),
				StatusId:     2,
			},
			setupMock: func(m *mockOrderService) {
				m.On("UpdateStatus", ctx, orderID, restID, domain.OrderStatus(2)).Return(domain.ErrNotFound)
			},
			expectedCode: codes.NotFound,
		},
		{
			name: "invalid status transition (FailedPrecondition)",
			req: &kitchenv1.UpdateOrderStatusRequest{
				OrderId:      orderID.String(),
				RestaurantId: restID.String(),
				StatusId:     4,
			},
			setupMock: func(m *mockOrderService) {
				m.On("UpdateStatus", ctx, orderID, restID, domain.OrderStatus(4)).Return(domain.ErrInvalidStatusChange)
			},
			expectedCode: codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcMock := new(mockOrderService)
			tt.setupMock(svcMock)

			srv := kitchengrpc.NewServer(svcMock, lgr)
			resp, err := srv.UpdateOrderStatus(ctx, tt.req)

			if tt.expectedCode == codes.OK {
				assert.NoError(t, err)
				assert.Equal(t, "status updated successfully", resp.GetMessage())
			} else {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			}
			svcMock.AssertExpectations(t)
		})
	}
}
