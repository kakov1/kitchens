package repo

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kitchens/internal/domain"
)

type OrderRepo struct {
	pg *Postgres
	qb sq.StatementBuilderType
}

func NewOrderRepo(pg *Postgres) *OrderRepo {
	return &OrderRepo{
		pg: pg,
		qb: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *OrderRepo) CreateOrder(ctx context.Context, order *domain.Order) error {
	tx, err := r.pg.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	orderQuery, orderArgs, err := r.qb.
		Insert("orders").
		Columns("id", "user_id", "restaurant_id", "status_id", "created_at", "updated_at").
		Values(order.ID, order.UserID, order.RestaurantID, order.StatusID, order.CreatedAt, order.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("building insert order query: %w", err)
	}

	if _, err := tx.Exec(ctx, orderQuery, orderArgs...); err != nil {
		return fmt.Errorf("inserting order: %w", err)
	}

	itemsBuilder := r.qb.
		Insert("order_items").
		Columns("id", "order_id", "menu_item_id", "quantity", "price_at_purchase")

	for _, item := range order.Items {
		itemsBuilder = itemsBuilder.Values(item.ID, item.OrderID, item.MenuItemID, item.Quantity, item.PriceAtPurchase)
	}

	itemQuery, itemArgs, err := itemsBuilder.ToSql()
	if err != nil {
		return fmt.Errorf("building insert order_items query: %w", err)
	}

	if _, err := tx.Exec(ctx, itemQuery, itemArgs...); err != nil {
		return fmt.Errorf("inserting order items: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *OrderRepo) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	query, args, err := r.qb.
		Select("id", "user_id", "restaurant_id", "status_id", "created_at", "updated_at").
		From("orders").
		Where(sq.Eq{"id": orderID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select order query: %w", err)
	}

	var order domain.Order
	err = r.pg.Pool.QueryRow(ctx, query, args...).Scan(
		&order.ID,
		&order.UserID,
		&order.RestaurantID,
		&order.StatusID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("querying order: %w", err)
	}

	return &order, nil
}

func (r *OrderRepo) GetOrdersByRestaurant(ctx context.Context, restaurantID uuid.UUID, statusID *domain.OrderStatus) ([]domain.Order, error) {
	builder := r.qb.
		Select("id", "user_id", "restaurant_id", "status_id", "created_at", "updated_at").
		From("orders").
		Where(sq.Eq{"restaurant_id": restaurantID}).
		OrderBy("created_at DESC")

	if statusID != nil {
		builder = builder.Where(sq.Eq{"status_id": *statusID})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	rows, err := r.pg.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying restaurant orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.RestaurantID, &o.StatusID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning order: %w", err)
		}
		orders = append(orders, o)
	}

	return orders, rows.Err()
}

func (r *OrderRepo) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, statusID domain.OrderStatus) error {
	query, args, err := r.qb.
		Update("orders").
		Set("status_id", statusID).
		Set("updated_at", sq.Expr("CURRENT_TIMESTAMP")).
		Where(sq.Eq{"id": orderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("building update order query: %w", err)
	}

	res, err := r.pg.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating order status: %w", err)
	}

	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
