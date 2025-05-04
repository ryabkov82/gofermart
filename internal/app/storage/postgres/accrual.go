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
	db                *sql.DB
	getOrdersStmt     *sql.Stmt
	updateOrderStmt   *sql.Stmt
	updateBalanceStmt *sql.Stmt
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

	getOrdersStmt, err := db.Prepare(`
	SELECT number, user_id, status, accrual, uploaded_at
	FROM orders
	WHERE status IN ('NEW', 'PROCESSING')
	AND number != ALL($1)
	ORDER BY uploaded_at ASC
	LIMIT $2
	`)

	if err != nil {
		return nil, fmt.Errorf("prepare orders statement failed: %w", err)
	}

	updateOrderStmt, err := db.Prepare(`
	UPDATE orders 
	SET status = $1, accrual = $2 
	WHERE number = $3
	`)

	if err != nil {
		return nil, fmt.Errorf("prepare order statement failed: %w", err)
	}

	updateBalanceStmt, err := db.Prepare(`
	INSERT INTO user_balances (user_id, current_balance, updated_at)
	VALUES ($1, $2, NOW())
	ON CONFLICT (user_id) DO UPDATE
	SET current_balance = user_balances.current_balance + EXCLUDED.current_balance,
		updated_at = NOW()
	`)

	if err != nil {
		return nil, fmt.Errorf("prepare balance statement failed: %w", err)
	}

	return &PostgresAccrualRepository{db: db, getOrdersStmt: getOrdersStmt, updateOrderStmt: updateOrderStmt, updateBalanceStmt: updateBalanceStmt}, nil
}

// GetOrdersForProcessing возвращает заказы для обработки
func (r *PostgresAccrualRepository) GetOrdersForProcessing(ctx context.Context, limit int, excludeOrders []string) ([]models.Order, error) {

	rows, err := r.getOrdersStmt.QueryContext(ctx, pq.Array(excludeOrders), limit)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
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
		for userID, accrual := range userAccruals {
			if _, err := r.updateBalanceStmt.ExecContext(ctx, userID, accrual); err != nil {
				return fmt.Errorf("failed to update balance for user %d: %w", userID, err)
			}
		}
	}

	// 2. Обновляем заказы (обычный UPDATE)
	if len(orderUpdates) > 0 {
		for _, order := range orderUpdates {
			if _, err := r.updateOrderStmt.ExecContext(ctx, order.Status, order.Accrual, order.Number); err != nil {
				return fmt.Errorf("update order failed for %s: %w", order.Number, err)
			}
		}
	}

	return tx.Commit()
}
