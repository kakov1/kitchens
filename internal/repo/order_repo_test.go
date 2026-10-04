package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kitchens/internal/domain"
	"kitchens/internal/repo"
)

func TestOrderRepo_Integration(t *testing.T) {
	pool, teardown := setupTestDB(t)
	defer teardown()

	pg := &repo.Postgres{Pool: pool}
	r := repo.NewOrderRepo(pg)
	ctx := context.Background()

	seedTestData := func(t *testing.T, userID, restID, menuID uuid.UUID) {
		t.Helper()
		email := "test_" + uuid.New().String() + "@test.com"
		_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'hash')", userID, email)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, "INSERT INTO restaurants (id, name, description) VALUES ($1, 'Test Rest', 'Desc') ON CONFLICT (id) DO NOTHING", restID)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, "INSERT INTO menu_items (id, restaurant_id, name, price, is_available) VALUES ($1, $2, 'Item', 100, true) ON CONFLICT (id) DO NOTHING", menuID, restID)
		require.NoError(t, err)
	}

	t.Run("CreateOrder", func(t *testing.T) {
		userID := uuid.New()
		restaurantID := uuid.New()
		menuItemID := uuid.New()
		seedTestData(t, userID, restaurantID, menuItemID)

		orderID := uuid.New()
		order := &domain.Order{
			ID:           orderID,
			UserID:       userID,
			RestaurantID: restaurantID,
			StatusID:     1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Items: []domain.OrderItem{
				{
					ID:              uuid.New(),
					OrderID:         orderID,
					MenuItemID:      menuItemID,
					Quantity:        2,
					PriceAtPurchase: 100,
				},
			},
		}

		err := r.CreateOrder(ctx, order)
		assert.NoError(t, err)

		var status int
		err = pool.QueryRow(ctx, "SELECT status_id FROM orders WHERE id = $1", orderID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, 1, status)
	})

	t.Run("GetOrderByID", func(t *testing.T) {
		userID := uuid.New()
		restaurantID := uuid.New()
		seedTestData(t, userID, restaurantID, uuid.New())

		orderID := uuid.New()
		_, err := pool.Exec(ctx, "INSERT INTO orders (id, user_id, restaurant_id, status_id) VALUES ($1, $2, $3, 1)", orderID, userID, restaurantID)
		require.NoError(t, err)

		order, err := r.GetOrderByID(ctx, orderID)
		assert.NoError(t, err)
		assert.Equal(t, orderID, order.ID)

		_, err = r.GetOrderByID(ctx, uuid.New())
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetOrdersByRestaurant", func(t *testing.T) {
		userID := uuid.New()
		restaurantID := uuid.New()

		email := "test_" + uuid.New().String() + "@test.com"
		_, err := pool.Exec(ctx, "INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'hash')", userID, email)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "INSERT INTO restaurants (id, name, description) VALUES ($1, 'Rest 2', 'Desc') ON CONFLICT (id) DO NOTHING", restaurantID)
		require.NoError(t, err)

		o1 := uuid.New()
		o2 := uuid.New()

		_, err = pool.Exec(ctx, "INSERT INTO orders (id, user_id, restaurant_id, status_id) VALUES ($1, $2, $3, 1), ($4, $5, $6, 2)", o1, userID, restaurantID, o2, userID, restaurantID)
		require.NoError(t, err)

		orders, err := r.GetOrdersByRestaurant(ctx, restaurantID, nil)
		assert.NoError(t, err)
		assert.Len(t, orders, 2)

		statusFilter := domain.OrderStatus(2)
		ordersFiltered, err := r.GetOrdersByRestaurant(ctx, restaurantID, &statusFilter)
		assert.NoError(t, err)
		require.Len(t, ordersFiltered, 1)
		assert.Equal(t, o2, ordersFiltered[0].ID)
	})

	t.Run("UpdateOrderStatus", func(t *testing.T) {
		userID := uuid.New()
		restaurantID := uuid.New()
		seedTestData(t, userID, restaurantID, uuid.New())

		orderID := uuid.New()
		_, err := pool.Exec(ctx, "INSERT INTO orders (id, user_id, restaurant_id, status_id) VALUES ($1, $2, $3, 1)", orderID, userID, restaurantID)
		require.NoError(t, err)

		err = r.UpdateOrderStatus(ctx, orderID, 3)
		assert.NoError(t, err)

		var status int
		err = pool.QueryRow(ctx, "SELECT status_id FROM orders WHERE id = $1", orderID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, 3, status)

		err = r.UpdateOrderStatus(ctx, uuid.New(), 3)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
