package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/service"
	"github.com/ryabkov82/gofermart/internal/app/storage"
)

type PostgresStorage struct {
	db                 *sql.DB
	insertUserStmt     *sql.Stmt
	getUserStmt        *sql.Stmt
	insertOrderStmt    *sql.Stmt
	getUserOrdersStmt  *sql.Stmt
	getUserBalanceStmt *sql.Stmt
}

func NewPostgresStorage(StoragePath string) (*PostgresStorage, error) {

	db, err := sql.Open("pgx", StoragePath)

	if err != nil {
		return nil, err
	}

	// Применяем миграции из текущего пакета
	if err := applyMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(50)
	db.SetConnMaxLifetime(5 * time.Minute)

	insertUserStmt, err := db.Prepare(`
	INSERT INTO users (login, password_hash)
	VALUES ($1, $2)
	ON CONFLICT (login) DO UPDATE SET
		login = EXCLUDED.login -- Фейковое обновление
	RETURNING id, xmax;
	`)

	if err != nil {
		return nil, err
	}

	getUserStmt, err := db.Prepare(`
	SELECT id, login, password_hash 
	FROM users 
	WHERE login = $1`)

	if err != nil {
		return nil, err
	}

	insertOrderStmt, err := db.Prepare(`
	INSERT INTO orders (number, user_id)
	VALUES ($1, $2)
	ON CONFLICT (number) DO UPDATE SET
		number = EXCLUDED.number  -- Фейковое обновление
	RETURNING user_id, xmax;
	`)

	if err != nil {
		return nil, err
	}

	getUserOrdersStmt, err := db.Prepare(`
	SELECT number, status, accrual, uploaded_at 
	FROM orders 
	WHERE user_id = $1 
	ORDER BY uploaded_at DESC`)

	if err != nil {
		return nil, err
	}

	getUserBalanceStmt, err := db.Prepare(`SELECT current_balance, withdrawn_balance FROM user_balances WHERE user_id = $1`)
	if err != nil {
		return nil, err
	}

	return &PostgresStorage{db, insertUserStmt, getUserStmt, insertOrderStmt, getUserOrdersStmt, getUserBalanceStmt}, nil

}

func (s *PostgresStorage) CreateUser(ctx context.Context, user *models.User) error {

	var xmax int64 // Системный столбец, показывающий был ли конфликт
	var userID int

	err := s.insertUserStmt.QueryRowContext(ctx, user.Login, user.PasswordHash).Scan(&userID, &xmax)

	if err != nil {
		return err
	}
	// Если xmax > 0, значит запись с login уже существовала (был конфликт)
	if xmax > 0 {
		err = storage.ErrLoginExists
	}

	user.UserID = userID

	return err
}

func (s *PostgresStorage) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {

	var user models.User
	err := s.getUserStmt.QueryRowContext(ctx, login).Scan(&user.UserID, &user.Login, &user.PasswordHash)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = storage.ErrLoginNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStorage) AddOrder(ctx context.Context, order *models.Order) error {

	var xmax int64 // Системный столбец, показывающий был ли конфликт
	var userID int

	err := s.insertOrderStmt.QueryRowContext(ctx, order.Number, order.UserID).Scan(&userID, &xmax)

	if err != nil {
		return err
	}
	// Если xmax > 0, значит запись с number уже существовала (был конфликт)
	if xmax > 0 {
		err = storage.ErrOrderExists
	}

	order.UserID = userID

	return err
}

func (s *PostgresStorage) GetUserOrders(ctx context.Context, userID int) ([]models.Order, error) {

	rows, err := s.getUserOrdersStmt.QueryContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil

}

func (s *PostgresStorage) GetUserBalance(ctx context.Context, userID int) (models.Balance, error) {

	balance := models.Balance{Current: 0, Withdrawn: 0}

	rows, err := s.getUserBalanceStmt.QueryContext(ctx, userID)
	if err != nil {
		return balance, err
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&balance.Current, &balance.Withdrawn); err != nil {
			return balance, err
		}
	}

	if err := rows.Err(); err != nil {
		return balance, fmt.Errorf("rows error: %w", err)
	}

	return balance, nil

}

func (s *PostgresStorage) WithdrawFunds(ctx context.Context, userID int, order string, sum float64) error {

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Проверяем текущий баланс
	var currentBalance float64
	err = tx.QueryRowContext(ctx,
		"SELECT current_balance FROM user_balances WHERE user_id = $1 FOR UPDATE",
		userID,
	).Scan(&currentBalance)

	if err != nil {
		return err
	}

	if currentBalance < sum {
		return service.ErrInsufficientFunds
	}

	// Обновляем баланс
	_, err = tx.ExecContext(ctx,
		"UPDATE user_balances SET current_balance = current_balance - $1, withdrawn_balance = withdrawn_balance + $1 WHERE user_id = $2",
		sum, userID,
	)
	if err != nil {
		return err
	}

	// Записываем операцию списания
	_, err = tx.ExecContext(ctx,
		"INSERT INTO withdrawals (user_id, order_number, sum, processed_at) VALUES ($1, $2, $3, $4)",
		userID, order, sum, time.Now(),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *PostgresStorage) GetWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error) {

	rows, err := s.db.QueryContext(ctx,
		"SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}
