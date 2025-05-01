package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ryabkov82/gofermart/internal/app/models"
	"github.com/ryabkov82/gofermart/internal/app/storage"
)

type PostgresStorage struct {
	db             *sql.DB
	insertUserStmt *sql.Stmt
	getUserStmt    *sql.Stmt
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
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

	return &PostgresStorage{db, insertUserStmt, getUserStmt}, nil

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
