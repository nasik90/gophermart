package pg

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nasik90/gophermart/internal/app/storage"
	"github.com/pressly/goose"
)

type Store struct {
	conn *sql.DB
}

func NewStore(conn *sql.DB) (*Store, error) {
	s := &Store{conn: conn}
	dir := "internal/migrations/pg"
	// Применение миграций
	migrationErr := goose.Up(conn, dir)
	//Откат миграций
	if migrationErr != nil {
		err := goose.Down(conn, dir)
		if err == nil {
			return s, migrationErr
		} else {
			return s, errors.Join(migrationErr, err)
		}
	}
	return s, nil
}

func (s Store) Close() error {
	return s.conn.Close()
}

func (s *Store) SaveNewUser(ctx context.Context, login, password string) error {
	_, err := s.conn.ExecContext(ctx, `INSERT INTO users (login, password) VALUES ($1, $2)`, login, password)
	err = saveNewUserCheckInsertError(err)
	return err
}

func saveNewUserCheckInsertError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		if pgErr.ConstraintName == "login_ukey" {
			return storage.ErrUserNotUnique
		}
	}
	return err
}

func (s *Store) UserIsValid(ctx context.Context, login, password string) (bool, error) {
	rows, err := s.conn.ExecContext(ctx, `SELECT FROM users WHERE login = $1 and password = $2`, login, password)
	if err != nil {
		return false, err
	}
	rowsAffected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}
	if rowsAffected > 0 {
		return true, nil
	}
	return false, nil
}

func (s *Store) SaveNewOrder(ctx context.Context, id int, login string) error {

	userID, err := s.getUserID(ctx, login)
	if err != nil {
		return err
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = createOrderWithStatusNew(ctx, tx, id, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func createOrderWithStatusNew(ctx context.Context, tx *sql.Tx, id int, userID int) error {
	uploadedAt := time.Now()

	if _, err := tx.ExecContext(ctx, `
		SAVEPOINT my_savepoint`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
        INSERT INTO orders (id, user_id, uploaded_at) VALUES ($1, $2, $3)`,
		id, userID, uploadedAt)

	err = saveNewOrderCheckInsertError(err)
	if err == storage.ErrOrderIDNotUnique {
		if _, err := tx.ExecContext(ctx, `
			ROLLBACK TO SAVEPOINT my_savepoint`); err != nil {
			return err
		}
		row := tx.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE id = $1`, id)
		var OrderUserID int
		if err := row.Scan(&OrderUserID); err != nil {
			return err
		}
		if OrderUserID != userID {
			return storage.ErrOrderLoadedByAnotherUser
		}
	}
	if err != nil {
		return err
	}

	return updateOrderStatus(ctx, tx, id, storage.StatusNEW, uploadedAt)
}

func updateOrderStatus(ctx context.Context, tx *sql.Tx, orderID int, statusID int, statusTime time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO history_statuses (date_time, order_id, status_id) VALUES ($1, $2, $3)`,
		statusTime, orderID, statusID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO current_statuses (order_id, status_id, date_time)
		VALUES ($1, $2, $3)
	 	ON CONFLICT (order_id)
		DO UPDATE SET status_id = $2, date_time = $3`,
		orderID, statusID, statusTime); err != nil {
		return err
	}
	return nil
}

func saveNewOrderCheckInsertError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		if pgErr.ConstraintName == "id_pkey" {
			return storage.ErrOrderIDNotUnique
		}
	}
	return err
}

func (s *Store) getUserID(ctx context.Context, login string) (int, error) {
	row := s.conn.QueryRowContext(ctx, `SELECT id FROM users WHERE login = $1`, login)
	var userID int
	if err := row.Scan(&userID); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *Store) getUserByOrder(ctx context.Context, OrderID int) (int, error) {
	row := s.conn.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE id = $1`, OrderID)
	var userID int
	if err := row.Scan(&userID); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *Store) GetOrderList(ctx context.Context, login string) (*[]storage.OrderData, error) {
	var result []storage.OrderData

	queryText :=
		`SELECT orders.id
			,COALESCE(status_values_kinds.name, '') as status
			,orders.uploaded_at
			,COALESCE(orders_points.points, 0) as accrual
		FROM orders
			INNER JOIN users
			ON orders.user_id = users.id
			LEFT JOIN current_statuses
			ON orders.id = current_statuses.order_id
			LEFT JOIN status_values_kinds
			ON current_statuses.status_id = status_values_kinds.id
			LEFT JOIN orders_points
			ON orders.id = orders_points.order_id
		WHERE users.login = $1
		`
	rows, err := s.conn.QueryContext(ctx, queryText, login)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		orderData := new(storage.OrderData)
		if err := rows.Scan(&orderData.Number, &orderData.Status, &orderData.UploadedAt, &orderData.Accrual); err != nil {
			return nil, err
		}
		result = append(result, *orderData)
	}

	if err := rows.Err(); err != nil {
		return &result, err
	}

	return &result, rows.Close()
}

// списание баллов
func (s *Store) WithdrawPoints(ctx context.Context, login string, OrderID int, points float64) error {
	userID, err := s.getUserID(ctx, login)
	if err != nil {
		return err
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT balance 
		FROM users_current_points
		WHERE user_id = $1 FOR UPDATE
	`, userID)
	if row.Err() != nil {
		return row.Err()
	}
	if err != nil {
		return err
	}
	balance := 0.0
	if err := row.Scan(&balance); err != nil {
		return err
	}
	if err := row.Err(); err != nil {
		return err
	}

	if balance < points {
		return storage.ErrOutOfBalance
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users_current_points SET points_out = points_out + $1, balance = balance - $1  WHERE user_id = $2 
	`, points, userID); err != nil {
		return err
	}

	// Создем заказ
	err = createOrderWithStatusNew(ctx, tx, OrderID, userID)
	if err != nil {
		return err
	}

	// Пишем в таблицу orders_points
	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders_points (date_time, order_id, user_id, flow_in, points) VALUES ($1, $2, $3, $4, $5)
		`, time.Now(), OrderID, userID, false, points)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// начисление баллов
func (s *Store) AccruePoints(ctx context.Context, orderID int, points float64) error {
	userID, err := s.getUserByOrder(ctx, orderID)
	if err != nil {
		return err
	}

	curTime := time.Now()

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users_current_points (user_id, points_in, points_out, balance) VALUES ($1, $2, $3, $4) 
			ON CONFLICT ON CONSTRAINT users_current_points_unique_order_id DO 
			UPDATE SET points_in = users_current_points.points_in + $2, balance = users_current_points.balance + $2  
			WHERE users_current_points.user_id = $1 
	`, userID, points, 0, points); err != nil {
		return err
	}

	// Пишем в таблицу orders_points
	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders_points (date_time, order_id, user_id, flow_in, points) VALUES ($1, $2, $3, $4, $5)
		`, curTime, orderID, userID, true, points)
	if err != nil {
		return err
	}

	if err := updateOrderStatus(ctx, tx, orderID, storage.StatusPROCESSED, curTime); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) GetUserBalance(ctx context.Context, login string) (*storage.UserBalance, error) {

	var result storage.UserBalance

	stmt, err := s.conn.PrepareContext(ctx,
		`SELECT  p.balance, p.points_out
			FROM users_current_points p
			INNER JOIN users u
			ON p.user_id = u.id
			WHERE u.login = $1`)

	if err != nil {
		return &result, err
	}

	row := stmt.QueryRowContext(ctx, login)
	if row.Err() != nil {
		return &result, err
	}
	if err := row.Scan(&result.Current, &result.Withdrawn); err != nil {
		return &result, err
	}
	return &result, nil

}

// список списаний
func (s *Store) GetWithdrawals(ctx context.Context, login string) (*[]storage.Withdrawals, error) {

	var result []storage.Withdrawals

	stmt, err := s.conn.PrepareContext(ctx,
		`SELECT date_time, order_id, points
			FROM orders_points o
			INNER JOIN users u
			ON o.user_id = u.id
		WHERE u.login = $1  and o.flow_in = false`)

	if err != nil {
		return &result, err
	}

	rows, err := stmt.QueryContext(ctx, login)
	if err != nil {
		return &result, err
	}
	defer rows.Close()
	for rows.Next() {
		withdrawals := new(storage.Withdrawals)
		if err := rows.Scan(&withdrawals.ProcessedAt, &withdrawals.Order, &withdrawals.Sum); err != nil {
			return nil, err
		}
		result = append(result, *withdrawals)
	}
	if err := rows.Err(); err != nil {
		return &result, err
	}
	return &result, rows.Close()
}

func (s *Store) SaveStatus(ctx context.Context, orderID int, statusID int) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateOrderStatus(ctx, tx, orderID, statusID, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) NewAndProcessingOrders(ctx context.Context) ([]int, error) {
	var result []int
	rows, err := s.conn.QueryContext(ctx, `SELECT order_id FROM current_statuses WHERE status_id IN ($1, $2) ORDER BY date_time ASC limit 1000`, storage.StatusNEW, storage.StatusPROCESSING)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var orderID int
		rows.Scan(&orderID)
		result = append(result, orderID)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	return result, rows.Close()
}
