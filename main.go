package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	createTable := `
	CREATE TABLE IF NOT EXISTS todos (
		id SERIAL PRIMARY KEY,
		todo TEXT,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
	);
	CREATE TABLE IF NOT EXISTS secrets (
		id SERIAL PRIMARY KEY,
		key TEXT
	);
	`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatal(err)
	}
}
