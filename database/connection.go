package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
	

	_ "github.com/lib/pq"
	
)

var (
	connString = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", os.Getenv("USER_BD"), os.Getenv("PASS_BD"), os.Getenv("HOST_BD"), os.Getenv("PORT_BD"), os.Getenv("DBNAME"))
	conex      *sql.DB
	ctx        context.Context
)

// Connect : function to connect the database of califications but no return the conection
func Connect() (conn *sql.Conn, err error) {
	if conex == nil {
		connString = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", os.Getenv("USER_BD"), os.Getenv("PASS_BD"), os.Getenv("HOST_BD"), os.Getenv("PORT_BD"), os.Getenv("DBNAME"))
		conex, err = sql.Open("postgres", connString)
		if err != nil {
			return
		}

		conex.SetConnMaxLifetime(time.Second * 30)
		conex.SetMaxIdleConns(0)
		conex.SetMaxOpenConns(200)
	}

	err = conex.Ping()
	if err != nil {
		return
	}

	ctx = context.TODO()
	conn, err = conex.Conn(ctx)
	if err != nil {
		return
	}

	return
}

// Query : function to make the query in the database
func Query(conn *sql.Conn, query string, data ...interface{}) (*sql.Rows, error) {
	return conn.QueryContext(ctx, query, data...)
}

// Exec : function to updates and deletes
func Exec(conn *sql.Conn, query string, data ...interface{}) (result sql.Result, err error) {
	return conn.ExecContext(ctx, query, data...)
}

// Close : function to close the connection with the database
func Close(conn *sql.Conn) error {
	return conn.Close()
}
