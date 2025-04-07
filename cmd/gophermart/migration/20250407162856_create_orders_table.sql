-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orders (
    id bigint CONSTRAINT id_pkey PRIMARY KEY NOT NULL,
    user_id int NOT NULL,
    uploaded_at timestamp NOT NULL
);
CREATE INDEX IF NOT EXISTS orders_user_id_idx ON orders (user_id)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE orders;
-- +goose StatementEnd