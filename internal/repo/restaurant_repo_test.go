package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kitchens/internal/repo"
)

func TestRestaurantRepo_Integration(t *testing.T) {
	pool, teardown := setupTestDB(t)
	defer teardown()

	ctx := context.Background()
	_, err := pool.Exec(ctx, "TRUNCATE TABLE restaurants, menu_items CASCADE")
	require.NoError(t, err)

	pg := &repo.Postgres{Pool: pool}
	r := repo.NewRestaurantRepo(pg)

	rest1ID := uuid.New()
	rest2ID := uuid.New()

	_, err = pool.Exec(ctx, "INSERT INTO restaurants (id, name, description) VALUES ($1, 'B Rest', 'Desc 2'), ($2, 'A Rest', 'Desc 1')", rest2ID, rest1ID)
	require.NoError(t, err)

	menuItem1ID := uuid.New()
	menuItem2ID := uuid.New()
	menuItem3ID := uuid.New()

	_, err = pool.Exec(ctx, "INSERT INTO menu_items (id, restaurant_id, name, price, is_available) VALUES "+
		"($1, $2, 'Pizza', 500, true), "+
		"($3, $4, 'Burger', 300, false), "+
		"($5, $6, 'Sushi', 700, true)",
		menuItem1ID, rest1ID, menuItem2ID, rest1ID, menuItem3ID, rest2ID)
	require.NoError(t, err)

	t.Run("GetRestaurants", func(t *testing.T) {
		rests, err := r.GetRestaurants(ctx)
		assert.NoError(t, err)
		require.Len(t, rests, 2)

		assert.Equal(t, rest1ID, rests[0].ID)
		assert.Equal(t, "A Rest", rests[0].Name)
		assert.Equal(t, rest2ID, rests[1].ID)
		assert.Equal(t, "B Rest", rests[1].Name)
	})

	t.Run("GetMenu", func(t *testing.T) {
		tests := []struct {
			name           string
			restaurantID   uuid.UUID
			expectedLength int
			expectedItemID uuid.UUID
		}{
			{
				name:           "Получение меню (только is_available=true)",
				restaurantID:   rest1ID,
				expectedLength: 1,
				expectedItemID: menuItem1ID,
			},
			{
				name:           "Меню второго ресторана",
				restaurantID:   rest2ID,
				expectedLength: 1,
				expectedItemID: menuItem3ID,
			},
			{
				name:           "Пустое меню для несуществующего ресторана",
				restaurantID:   uuid.New(),
				expectedLength: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				items, err := r.GetMenu(ctx, tt.restaurantID)
				assert.NoError(t, err)
				require.Len(t, items, tt.expectedLength)
				if tt.expectedLength > 0 {
					assert.Equal(t, tt.expectedItemID, items[0].ID)
				}
			})
		}
	})

	t.Run("GetMenuItemsByIDs", func(t *testing.T) {
		items, err := r.GetMenuItemsByIDs(ctx, []uuid.UUID{menuItem1ID, menuItem2ID})
		assert.NoError(t, err)

		require.Len(t, items, 2)

		var resultIDs []uuid.UUID
		for _, item := range items {
			resultIDs = append(resultIDs, item.ID)
		}
		assert.Contains(t, resultIDs, menuItem1ID)
		assert.Contains(t, resultIDs, menuItem2ID)
	})
}
