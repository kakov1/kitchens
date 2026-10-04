package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const OrderCreatedTopic = "kitchen.orders"

type OrderCreatedEvent struct {
	OrderID      uuid.UUID `json:"order_id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type OrderProducer struct {
	writer *kafka.Writer
	lgr    *slog.Logger
}

func NewOrderProducer(brokers []string, lgr *slog.Logger) *OrderProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        OrderCreatedTopic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 5 * time.Second,
		RequiredAcks: kafka.RequireOne,
	}

	return &OrderProducer{
		writer: writer,
		lgr:    lgr,
	}
}

func (p *OrderProducer) PublishOrderCreated(ctx context.Context, orderID, restaurantID uuid.UUID) error {
	event := OrderCreatedEvent{
		OrderID:      orderID,
		RestaurantID: restaurantID,
		CreatedAt:    time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal order event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(restaurantID.String()),
		Value: payload,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.lgr.Error("failed to publish kafka event", slog.String("error", err.Error()))
		return fmt.Errorf("kafka write message: %w", err)
	}

	p.lgr.Info("published order.created event to kafka", slog.String("order_id", orderID.String()))
	return nil
}

func (p *OrderProducer) Close() error {
	return p.writer.Close()
}
