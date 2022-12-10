package main

import (
	"database/sql"
	"fmt"
)

type PostgresDBParams struct {
	dbName   string
	host     string
	user     string
	password string
}

type PostgresTransactionLogger struct {
	events chan<- Event
	errors <-chan error
	db     *sql.DB
}

func (l *PostgresTransactionLogger) WritePut(key, value string) {
	l.events <- Event{EventType: EventPut, Key: key, Value: value}
}

func (l *PostgresTransactionLogger) WriteDelete(key string) {
	l.events <- Event{EventType: EventDelete, Key: key}
}

func (l *PostgresTransactionLogger) Err() <-chan error {
	return l.errors
}

func (l *PostgresTransactionLogger) verifyTableExists() (bool, error) {
	const table = "transactions"
	var result string
	rows, err := l.db.Query(fmt.Sprintf("SELECT to_regclass('public.%s');", table))
	defer rows.Close()
	if err != nil {
		return false, err
	}
	for rows.Next() && result != table {
		rows.Scan(&result)
	}

	return result == table, rows.Err()
}

func (l *PostgresTransactionLogger) createTable() error {
	var err error

	createQuery := `CREATE TABLE transactions (
			sequence BIGSERIAL PRIMARY KEY
			event_type SMALLINT
			key	TEXT
			value TEXT
		);`
	_, err = l.db.Exec(createQuery)
	if err != nil {
		return err
	}
	return nil
}

func NewPostgresTransactionLogger(config PostgresDBParams) (TransactionLogger, error) {
	connStr := fmt.Sprintf("host=%s dbname=%s user=%s password =%s", config.host, config.dbName, config.user, config.password)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to open db connection: %w", err)
	}

	logger := &PostgresTransactionLogger{db: db}
	exists, err := logger.verifyTableExists()
	if err != nil {
		return nil, fmt.Errorf("failed to verify table exists: %w", err)
	}
	if !exists {
		if err := logger.createTable(); err != nil {
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return logger, nil

}
