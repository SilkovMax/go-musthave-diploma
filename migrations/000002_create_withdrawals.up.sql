CREATE TABLE IF NOT EXISTS withdrawals (
    id SERIAL PRIMARY KEY,
    user_login VARCHAR(255) NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    order_number VARCHAR(255) NOT NULL UNIQUE,
    sum NUMERIC(10, 2) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user_login ON withdrawals(user_login);