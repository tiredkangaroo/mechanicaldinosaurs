package main

import (
	"context"
	"log/slog"
	"os"

	"database/sql"

	database "github.com/tiredkangaroo/mechanicaldinosaurs/console-api/db"
	_ "modernc.org/sqlite"
)

var DATABASE_PATH = "console.db"  // NOTE: replace with env var later
var SCHEMA_PATH = "db/schema.sql" // NOTE: embed or env var later

func main() {
	_, err := os.Stat(DATABASE_PATH)
	isNewDatabase := os.IsNotExist(err)

	conn, err := sql.Open("sqlite", "file:"+DATABASE_PATH)
	if err != nil {
		slog.Error("open database", "error", err)
		return
	}
	if isNewDatabase { // new db: run the schema creation script
		slog.Info("initializing new database")
		schema, err := os.ReadFile(SCHEMA_PATH)
		if err != nil {
			slog.Error("read schema file", "error", err)
			return
		}
		_, err = conn.Exec(string(schema))
		if err != nil {
			slog.Error("execute schema", "error", err)
			return
		}
	}
	db := database.New(conn)

	err = db.AddRemoteServer(context.TODO(), database.AddRemoteServerParams{
		Name:     "pineapple",
		Hostport: "pineapple:6731",
		Secret:   "grr",
	})
	if err != nil {
		slog.Error("add remote server", "error", err)
		return
	}

	remoteServers, err := db.ListRemoteServers(context.TODO())
	if err != nil {
		slog.Error("list remote servers", "error", err)
		return
	}
	for _, server := range remoteServers {
		slog.Info("remote server", "name", server.Name, "hp", server.Hostport)
	}
}
