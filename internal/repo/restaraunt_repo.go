package repo

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"kitchens/internal/domain"
)

type RestaurantRepo struct {
	pg *Postgres
	qb sq.StatementBuilderType
}

func NewRestaurantRepo(pg *Postgres) *RestaurantRepo {
	return &RestaurantRepo{
		pg: pg,
		qb: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *RestaurantRepo) GetRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	query, args, err := r.qb.
		Select("id", "name", "description", "created_at").
		From("restaurants").
		OrderBy("name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	rows, err := r.pg.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying restaurants: %w", err)
	}
	defer rows.Close()

	var restaurants []domain.Restaurant
	for rows.Next() {
		var rest domain.Restaurant
		if err := rows.Scan(&rest.ID, &rest.Name, &rest.Description, &rest.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning restaurant: %w", err)
		}
		restaurants = append(restaurants, rest)
	}

	return restaurants, rows.Err()
}

func (r *RestaurantRepo) GetMenu(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error) {
	query, args, err := r.qb.
		Select("id", "restaurant_id", "name", "price", "is_available").
		From("menu_items").
		Where(sq.Eq{"restaurant_id": restaurantID, "is_available": true}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	rows, err := r.pg.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying menu items: %w", err)
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		if err := rows.Scan(&item.ID, &item.RestaurantID, &item.Name, &item.Price, &item.IsAvailable); err != nil {
			return nil, fmt.Errorf("scanning menu item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *RestaurantRepo) GetMenuItemsByIDs(ctx context.Context, itemIDs []uuid.UUID) ([]domain.MenuItem, error) {
	query, args, err := r.qb.
		Select("id", "restaurant_id", "name", "price", "is_available").
		From("menu_items").
		Where(sq.Eq{"id": itemIDs}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	rows, err := r.pg.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying items by ids: %w", err)
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		if err := rows.Scan(&item.ID, &item.RestaurantID, &item.Name, &item.Price, &item.IsAvailable); err != nil {
			return nil, fmt.Errorf("scanning item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}
