package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"go-web-gin-health/internal/schema"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "schema-migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		return fmt.Errorf("MYSQL_DSN must select the target MySQL database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open MySQL connection: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to selected MySQL database: %w", err)
	}
	result, err := schema.ApplyLatest(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("managed schema through %s (v1 %s): %s\n", schema.PlatformExpenseReversalsV8Version, schema.BaselineVersion, result)
	return nil
}
