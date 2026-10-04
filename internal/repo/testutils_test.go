package repo_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	dbConnStr := os.Getenv("DB_CONN")
	if dbConnStr == "" {
		dbConnStr = "postgres://e2e_user:e2e_password@localhost:5432/avito_kitchen_e2e?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbConnStr)
	if err != nil {
		t.Skipf("skipping integration test: cannot parse pool conn: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: database %s is not available: %v", dbConnStr, err)
	}

	_, err = pool.Exec(ctx, "TRUNCATE TABLE orders, order_items, users, restaurants CASCADE")
	if err != nil {
		pool.Close()
		t.Skipf("skipping integration test: truncate failed: %v", err)
	}

	teardown := func() {
		pool.Close()
	}
	return pool, teardown
}
