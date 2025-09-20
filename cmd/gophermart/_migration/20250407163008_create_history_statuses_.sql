-- +goose Up
-- +goose StatementBegin
-- таблица для хранения истории статусов
CREATE TABLE IF NOT EXISTS history_statuses
(
    date_time timestamp NOT NULL,
    order_id bigint NOT NULL,
    status_id int NOT NULL,
    CONSTRAINT unique_key UNIQUE (date_time, order_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE history_statuses;
-- +goose StatementEnd
