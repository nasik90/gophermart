-- +goose Up
-- +goose StatementBegin
-- таблица для движений баллов
-- flow_in - true, если поступление баллов, false, если расход
-- point - количество баллов
CREATE TABLE IF NOT EXISTS orders_points
(
    date_time timestamp NOT NULL,
    order_id bigint NOT NULL,
    user_id bigint NOT NULL,
    flow_in boolean DEFAULT false,
    points numeric NOT NULL,
    CONSTRAINT orders_points_unique_key UNIQUE (order_id, flow_in)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE orders_points;
-- +goose StatementEnd
