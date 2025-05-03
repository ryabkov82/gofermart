BEGIN;

-- Таблица для хранения баланса пользователей
CREATE TABLE user_balances (
    user_id INTEGER PRIMARY KEY,
    current_balance DECIMAL(10, 2) NOT NULL DEFAULT 0,
    withdrawn_balance DECIMAL(10, 2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Добавим колонку accrual в таблицу orders, если её нет
ALTER TABLE orders ADD COLUMN IF NOT EXISTS accrual DECIMAL(10, 2) DEFAULT 0;

COMMIT;
