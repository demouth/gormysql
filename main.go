package gormysql

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type (
	DB struct {
		db *sql.DB
	}
)

func Open(source string) (db DB, err error) {
	db.db, err = sql.Open("mysql", source)
	return
}
