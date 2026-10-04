package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	kitchenv1 "kitchens/pkg/api/kitchen/v1"
)

type OrderCreatedEvent struct {
	OrderID      uuid.UUID `json:"order_id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func main() {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	grpcAddr := os.Getenv("KITCHEN_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = "localhost:9000"
	}

	kafkaBrokersStr := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}

	targetRestIDStr := os.Getenv("RESTAURANT_ID")
	if targetRestIDStr == "" {
		targetRestIDStr = "00000000-0000-0000-0000-000000000001"
	}
	targetRestID, _ := uuid.Parse(targetRestIDStr)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		lgr.Error("failed to connect to grpc server", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = conn.Close() }()
	kitchenClient := kitchenv1.NewKitchenServiceClient(conn)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBrokersStr},
		Topic:          "kitchen.orders",
		GroupID:        "kitchen-consumer-group",
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	defer func() { _ = reader.Close() }()

	lgr.Info("restaurant service started with Kafka & gRPC",
		slog.String("grpc", grpcAddr),
		slog.String("kafka", kafkaBrokersStr),
		slog.String("restaurant_id", targetRestID.String()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumeOrders(ctx, reader, kitchenClient, targetRestID, lgr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	lgr.Info("shutting down restaurant service...")
	cancel()
	time.Sleep(1 * time.Second)
}

func consumeOrders(ctx context.Context, reader *kafka.Reader, client kitchenv1.KitchenServiceClient, restID uuid.UUID, lgr *slog.Logger) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return
			}
			lgr.Error("failed to fetch message from kafka", slog.String("error", err.Error()))
			time.Sleep(1 * time.Second)
			continue
		}

		var event OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			lgr.Error("failed to unmarshal kafka event", slog.String("error", err.Error()))
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		lgr.Info("kafka message received",
			slog.String("order_id", event.OrderID.String()),
			slog.String("event_restaurant_id", event.RestaurantID.String()),
			slog.String("target_restaurant_id", restID.String()),
		)

		if event.RestaurantID == restID {
			lgr.Info("handling cooking lifecycle for order", slog.String("order_id", event.OrderID.String()))
			go handleCookingLifecycle(client, event.OrderID.String(), restID.String(), lgr)
		}

		_ = reader.CommitMessages(ctx, msg)
	}
}

func handleCookingLifecycle(client kitchenv1.KitchenServiceClient, orderID, restID string, lgr *slog.Logger) {
	ctx := context.Background()

	_, err := client.UpdateOrderStatus(ctx, &kitchenv1.UpdateOrderStatusRequest{
		OrderId:      orderID,
		RestaurantId: restID,
		StatusId:     2,
	})
	if err != nil {
		lgr.Error("failed to set status accepted via gRPC", slog.String("order_id", orderID), slog.String("error", err.Error()))
		return
	}
	lgr.Info("order accepted via gRPC", slog.String("order_id", orderID))

	time.Sleep(2 * time.Second)

	_, err = client.UpdateOrderStatus(ctx, &kitchenv1.UpdateOrderStatusRequest{
		OrderId:      orderID,
		RestaurantId: restID,
		StatusId:     3,
	})
	if err != nil {
		lgr.Error("failed to set status cooking via gRPC", slog.String("order_id", orderID), slog.String("error", err.Error()))
		return
	}
	lgr.Info("order is cooking via gRPC", slog.String("order_id", orderID))

	time.Sleep(8 * time.Second)

	_, err = client.UpdateOrderStatus(ctx, &kitchenv1.UpdateOrderStatusRequest{
		OrderId:      orderID,
		RestaurantId: restID,
		StatusId:     4,
	})
	if err != nil {
		lgr.Error("failed to set status ready via gRPC", slog.String("order_id", orderID), slog.String("error", err.Error()))
		return
	}
	lgr.Info("order is ready via gRPC!", slog.String("order_id", orderID))
}
