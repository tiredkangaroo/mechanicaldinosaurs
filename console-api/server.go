package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"database/sql"

	"github.com/labstack/echo/v4"
	database "github.com/tiredkangaroo/mechanicaldinosaurs/console-api/db"
	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
	_ "modernc.org/sqlite"
)

var DATABASE_PATH = "console.db"  // NOTE: replace with env var later
var SCHEMA_PATH = "db/schema.sql" // NOTE: embed or env var later
var SEED_PATH = "db/seed.sql"     // NOTE: embed or env var later
var PORT = "3742"                 // NOTE: replace with env var later

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
		seed, err := os.ReadFile(SEED_PATH)
		if err != nil {
			slog.Error("read seed file", "error", err)
			return
		}
		_, err = conn.Exec(string(seed))
		if err != nil {
			slog.Error("execute seed", "error", err)
			return
		}
	}

	db := database.New(conn)
	app := echo.New()

	api := app.Group("/api")

	api.Use(createAuthMiddleware(db))

	api.GET("/api/users", func(c echo.Context) error {
		users, err := db.ListUsers(c.Request().Context())
		if err != nil {
			slog.Error("list users", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		var names []string
		for _, u := range users {
			names = append(names, u.Name)
		}
		return c.JSON(200, names)
	})

	api.POST("/api/users", func(c echo.Context) error {
		if user := c.Get("user").(*database.User); !user.Superuser {
			return c.JSON(403, map[string]string{"error": "forbidden"})
		}
		var createUserRequest database.AddUserParams
		if err := c.Bind(&createUserRequest); err != nil {
			slog.Error("bind request body", "error", err)
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if createUserRequest.Name == "" || createUserRequest.TotpSecret == "" {
			return c.JSON(400, map[string]string{"error": "name and totp_secret are required"})
		}
		if err = db.AddUser(c.Request().Context(), createUserRequest); err != nil {
			slog.Error("create user", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		return c.JSON(201, nil)
	})

	// NOTE: implement global rate limited login
	api.POST("/login", func(c echo.Context) error {
		var loginRequest struct {
			Name string `json:"name"`
			Code string `json:"code"`
		}
		if err := c.Bind(&loginRequest); err != nil {
			slog.Error("bind request body", "error", err)
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if err := checkTOTP(c.Request().Context(), db, loginRequest.Name, loginRequest.Code); err != nil {
			slog.Error("check totp", "name", loginRequest.Name, "error", err)
			return c.JSON(401, map[string]string{"error": "invalid credentials"})
		}
		token, err := issueJWT(loginRequest.Name)
		if err != nil {
			slog.Error("issue JWT", "name", loginRequest.Name, "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		c.SetCookie(&http.Cookie{
			Name:    "auth",
			Value:   token,
			Expires: time.Now().Add(INACTIVITY_TIMEOUT),
			// Secure: true, // NOTE: again enable when we have https
			HttpOnly: true,
		})
		return c.JSON(200, nil)
	})

	api.GET("/logout", func(c echo.Context) error {
		c.SetCookie(&http.Cookie{
			Name:     "auth",
			Value:    "",
			Expires:  time.Now().Add(-time.Hour), // expire in the past to delete
			MaxAge:   -1,
			HttpOnly: true,
		})
		return c.JSON(200, nil)
	})

	api.GET("/api/machines", func(c echo.Context) error {
		machines, err := db.ListRemoteServers(c.Request().Context())
		if err != nil {
			slog.Error("list machines", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		var resp []MachineWithStatus
		for _, m := range machines {
			var mstatus MachineWithStatus
			mstatus.Name = m.Name
			mstatus.Hostport = m.Hostport

			info, err := server.Call(server.GetInfoRequest, m.Hostport, server.NoRequestData, m.Secret)
			if err != nil {
				slog.Error("get machine info", "name", m.Name, "error", err)
			} else {
				mstatus.Status = info
			}
			resp = append(resp, mstatus)
		}
		return c.JSON(200, resp)
	})

	api.GET("/api/machines/:name", func(c echo.Context) error {
		name := c.Param("name")
		machine, err := db.GetRemoteServer(c.Request().Context(), name)
		if err != nil {
			slog.Error("get machine", "name", name, "error", err)
			return c.JSON(404, map[string]string{"error": "machine not found"})
		}
		var resp MachineWithStatus
		resp.Name = machine.Name
		resp.Hostport = machine.Hostport

		info, err := server.Call(server.GetInfoRequest, machine.Hostport, server.NoRequestData, machine.Secret)
		if err != nil {
			slog.Error("get machine info", "name", machine.Name, "error", err)
		} else {
			resp.Status = info
		}

		return c.JSON(200, resp)
	})

	api.POST("/api/machines", func(c echo.Context) error {
		var remoteServer database.AddRemoteServerParams
		if err := c.Bind(&remoteServer); err != nil {
			slog.Error("bind request body", "error", err)
			return c.JSON(400, map[string]string{"error": "invalid request body"})
		}
		if remoteServer.Name == "" || remoteServer.Hostport == "" || remoteServer.Secret == "" {
			return c.JSON(400, map[string]string{"error": "name, hostport, and secret are required"})
		}
		err = db.AddRemoteServer(c.Request().Context(), remoteServer)
		if err != nil {
			slog.Error("create machine", "error", err)
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}
		return c.JSON(201, nil)
	})

	// NOTE: allow for certs
	app.Start(":" + PORT)
}

type MachineWithStatus struct {
	Name     string       `json:"name"`
	Hostport string       `json:"hostport"`
	Status   *server.Info `json:"status"`
}
