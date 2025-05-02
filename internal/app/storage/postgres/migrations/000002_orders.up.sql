BEGIN;

CREATE TYPE order_status AS ENUM (
    'NEW',          -- Новый заказ
    'PROCESSING',   -- В обработке
    'INVALID',      -- Не прошел обработку
    'PROCESSED'     -- Обработан успешно
);

CREATE TABLE IF NOT EXISTS orders (
    number TEXT PRIMARY KEY,          -- Номер заказа (основной ключ)
    user_id INTEGER NOT NULL,         -- ID пользователя, владельца заказа
    status order_status NOT NULL DEFAULT 'NEW', -- Статус заказа
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW() -- Время загрузки);
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);

COMMIT;         