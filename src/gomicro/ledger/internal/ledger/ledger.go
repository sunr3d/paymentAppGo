package ledger

import "database/sql"

func Insert(db *sql.DB, orderID string, userID string, amount int64, operation string, date string) error {
	stmt, err := db.Prepare("INSERT INTO transaction(order_id, user_id, amount, operation, date) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(orderID, userID, amount, operation, date)
	if err != nil {
		return err
	}

	return nil
}
