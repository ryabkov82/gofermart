package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"

	"github.com/ryabkov82/gofermart/internal/app/models"
)

type PostgresAccrualRepository struct {
	db *sql.DB
}

// NewPostgresAccrualRepository создает новый репозиторий для PostgreSQL
func NewPostgresAccrualRepository(StoragePath string) (*PostgresAccrualRepository, error) {

	db, err := sql.Open("pgx", StoragePath)

	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresAccrualRepository{db: db}, nil
}

// GetOrdersForProcessing возвращает заказы для обработки
func (r *PostgresAccrualRepository) GetOrdersForProcessing(ctx context.Context, limit int, excludeOrders []string) ([]models.Order, error) {

	query := `
        SELECT number, user_id, status, accrual, uploaded_at
        FROM orders
        WHERE status IN ('NEW', 'PROCESSING')
        AND number != ALL($1)
        ORDER BY uploaded_at ASC
        LIMIT $2
    `

	rows, err := r.db.QueryContext(ctx, query, pq.Array(excludeOrders), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.Number, &o.UserID, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	return orders, nil
}

func (r *PostgresAccrualRepository) ProcessAccruals(ctx context.Context, userAccruals map[int]float64, orderUpdates []models.Order) error {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Обновляем балансы (атомарный UPSERT)
	if len(userAccruals) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
            INSERT INTO user_balances (user_id, current_balance, updated_at)
            VALUES ($1, $2, NOW())
            ON CONFLICT (user_id) DO UPDATE
            SET current_balance = user_balances.current_balance + EXCLUDED.current_balance,
                updated_at = NOW()
        `)
		if err != nil {
			return fmt.Errorf("prepare balance statement failed: %w", err)
		}
		defer stmt.Close()

		for userID, accrual := range userAccruals {
			if _, err := stmt.ExecContext(ctx, userID, accrual); err != nil {
				return fmt.Errorf("failed to update balance for user %d: %w", userID, err)
			}
		}
	}

	// 2. Обновляем заказы (обычный UPDATE)
	if len(orderUpdates) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
            UPDATE orders 
            SET status = $1, accrual = $2 
            WHERE number = $3
        `)
		if err != nil {
			return fmt.Errorf("prepare order statement failed: %w", err)
		}
		defer stmt.Close()

		for _, order := range orderUpdates {
			if _, err := stmt.ExecContext(ctx, order.Status, order.Accrual, order.Number); err != nil {
				return fmt.Errorf("update order failed for %s: %w", order.Number, err)
			}
		}
	}

	return tx.Commit()
}
