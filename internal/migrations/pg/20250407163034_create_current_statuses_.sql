-- +goose Up
-- +goose StatementBegin
-- таблица для хранения текущего статуса
CREATE TABLE IF NOT EXISTS current_statuses
(
    order_id bigint NOT NULL,
    status_id int NOT NULL,
    date_time timestamp NOT NULL,
    CONSTRAINT current_statuses_unique_key UNIQUE (order_id)
);
CREATE INDEX IF NOT EXISTS current_statuses_status_id_idx ON current_statuses (status_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE current_statuses;
-- +goose StatementEnd
