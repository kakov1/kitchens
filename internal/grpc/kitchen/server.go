package kitchen

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"kitchens/internal/domain"
	kitchenv1 "kitchens/pkg/api/kitchen/v1"
)

type OrderService interface {
	GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error)
	UpdateStatus(ctx context.Context, orderID, restaurantID uuid.UUID, newStatus domain.OrderStatus) error
}

type Server struct {
	kitchenv1.UnimplementedKitchenServiceServer
	svc OrderService
	lgr *slog.Logger
}

func NewServer(svc OrderService, lgr *slog.Logger) *Server {
	return &Server{
		svc: svc,
		lgr: lgr,
	}
}

func (s *Server) GetOrders(ctx context.Context, req *kitchenv1.GetOrdersRequest) (*kitchenv1.GetOrdersResponse, error) {
	restID, err := uuid.Parse(req.GetRestaurantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid restaurant_id UUID")
	}

	var filterStatus *domain.OrderStatus
	if req.GetStatusId() > 0 {
		st := domain.OrderStatus(req.GetStatusId())
		filterStatus = &st
	}

	orders, err := s.svc.GetOrdersByRestaurant(ctx, restID, filterStatus)
	if err != nil {
		s.lgr.Error("failed to get restaurant orders via grpc", slog.String("error", err.Error()))
		return nil, status.Error(codes.Internal, "failed to get orders")
	}

	resp := &kitchenv1.GetOrdersResponse{
		Orders: make([]*kitchenv1.Order, 0, len(orders)),
	}

	for _, o := range orders {
		pbItems := make([]*kitchenv1.OrderItem, 0, len(o.Items))
		for _, it := range o.Items {
			pbItems = append(pbItems, &kitchenv1.OrderItem{
				Id:              it.ID.String(),
				MenuItemId:      it.MenuItemID.String(),
				Quantity:        int32(it.Quantity),
				PriceAtPurchase: int32(it.PriceAtPurchase),
			})
		}

		resp.Orders = append(resp.Orders, &kitchenv1.Order{
			Id:           o.ID.String(),
			UserId:       o.UserID.String(),
			RestaurantId: o.RestaurantID.String(),
			StatusId:     int32(o.StatusID),
			CreatedAt:    o.CreatedAt.String(),
			Items:        pbItems,
		})
	}

	return resp, nil
}

func (s *Server) UpdateOrderStatus(ctx context.Context, req *kitchenv1.UpdateOrderStatusRequest) (*kitchenv1.UpdateOrderStatusResponse, error) {
	orderID, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id UUID")
	}

	restID, err := uuid.Parse(req.GetRestaurantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid restaurant_id UUID")
	}

	newStatus := domain.OrderStatus(req.GetStatusId())

	err = s.svc.UpdateStatus(ctx, orderID, restID, newStatus)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		if errors.Is(err, domain.ErrInvalidStatusChange) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		s.lgr.Error("failed to update order status via grpc", slog.String("error", err.Error()))
		return nil, status.Error(codes.Internal, "failed to update order status")
	}

	return &kitchenv1.UpdateOrderStatusResponse{
		Message: "status updated successfully",
	}, nil
}
