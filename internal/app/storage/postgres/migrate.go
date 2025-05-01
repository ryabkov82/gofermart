package postgres

import (
	"database/sql"
	"embed"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var fs embed.FS

func applyMigrations(db *sql.DB) error {
	// 1. Создаем драйвер для источника миграций (из embed.FS)
	sourceDriver, err := iofs.New(fs, "migrations")
	if err != nil {
		return err
	}

	// 2. Создаем драйвер для базы данных
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{
		StatementTimeout: 5 * time.Minute, // Для операций миграции
	})
	if err != nil {
		return err
	}

	// 3. Инициализируем мигратор
	m, err := migrate.NewWithInstance(
		"iofs",       // Имя драйвера источника
		sourceDriver, // Экземпляр драйвера источника
		"postgres",   // Имя драйвера БД
		dbDriver)     // Экземпляр драйвера БД
	if err != nil {
		return err
	}

	// 4. Применяем миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
