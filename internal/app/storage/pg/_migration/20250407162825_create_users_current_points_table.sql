-- +goose Up
-- +goose StatementBegin
-- таблица для остатков по покупателю
CREATE TABLE IF NOT EXISTS users_current_points
    (
        user_id int NOT NULL,
        points_in numeric NOT NULL,
        points_out numeric NOT NULL,
        balance numeric NOT NULL,
        CONSTRAINT users_current_points_unique_order_id UNIQUE (user_id)
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users_current_points;
-- +goose StatementEnd
