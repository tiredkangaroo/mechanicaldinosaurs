package main

import (
	"context"
	"log/slog"
	"os"

	"database/sql"

	_ "embed"

	"github.com/labstack/echo/v4"
	database "github.com/tiredkangaroo/mechanicaldinosaurs/console-api/db"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	_ "modernc.org/sqlite"
)

var DATABASE_PATH = "console.db" // NOTE: replace with env var later
//go:embed db/schema.sql
var SCHEMA_PATH string

var PORT = "3742" // NOTE: replace with env var later

func main() {
	_, err := os.Stat(DATABASE_PATH)
	isNewDatabase := os.IsNotExist(err)

	conn, err := sql.Open("sqlite", "file:"+DATABASE_PATH)
	if err != nil {
		slog.Error("open database", "error", err)
		return
	}
	db := database.New(conn)
	if isNewDatabase { // new db: run the schema creation script
		ctx := context.Background()
		slog.Info("initializing new database")
		schema, err := os.ReadFile(SCHEMA_PATH)
		if err != nil {
			slog.Error("read schema file", "error", err)
			return
		}
		_, err = conn.ExecContext(ctx, string(schema))
		if err != nil {
			slog.Error("execute schema", "error", err)
			return
		}
		if err := db.AddUser(ctx, database.AddUserParams{ // add the starting user
			Name:       DefaultConfig.STARTING_USER_NAME,
			TotpSecret: DefaultConfig.STARTING_USER_TOTP_SECRET,
			Active:     true,
			Superuser:  true,
		}); err != nil {
			slog.Error("add starting user", "error", err)
			return
		}
	}

	app := echo.New()

	api := app.Group("/api")

	api.Use(createAuthMiddleware(db))

	addAuthRoutes(api, db)
	addMachineRoutes(api, db)

	if DefaultConfig.CERT_PATH == "" || DefaultConfig.KEY_PATH == "" {
		slog.Warn("no tls certificate or key provided, starting server without tls")
		if err := app.Start(":" + PORT); err != nil {
			slog.Error("server", "error", err)
			return
		}
	} else {
		slog.Info("starting server with tls", "cert", DefaultConfig.CERT_PATH, "key", DefaultConfig.KEY_PATH)
		if err := app.StartTLS(":"+PORT, DefaultConfig.CERT_PATH, DefaultConfig.KEY_PATH); err != nil {
			slog.Error("server", "error", err)
			return
		}
	}
}

type MachineWithStatus struct {
	Name     string       `json:"name"`
	Hostport string       `json:"hostport"`
	Status   *server.Info `json:"status"`
}
