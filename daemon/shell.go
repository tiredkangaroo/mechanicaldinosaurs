// this file had significant ai assistance in generating the code
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type WindowSize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func handleTerminal(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	// start pty w bash
	// this probably just spawns a shell with the same user as the daemon
	// and the daemon should be running on root so the shell will be root as well..... that's fine ig
	cmd := exec.Command("/bin/bash")
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		log.Println("PTY start error:", err)
		return
	}
	defer func() {
		_ = ptyFile.Close()
		_ = cmd.Process.Kill()
	}()

	// pipe stdout/stderr to the ws conn
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptyFile.Read(buf)
			if err != nil {
				return
			}
			if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// pipe ws conn to stdin (also handle resize control frames)
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}

		// is json for resize?
		var size WindowSize
		if err := json.Unmarshal(msg, &size); err == nil && size.Cols > 0 && size.Rows > 0 {
			_ = pty.Setsize(ptyFile, &pty.Winsize{Rows: size.Rows, Cols: size.Cols})
			continue
		}

		// otherwise, write to stdin
		_, _ = ptyFile.Write(msg)
	}
}

func registerShellRoutes(api *echo.Group) {
	// api middleware will check auth so shell should be protected lol
	api.GET("/api/shell", func(c echo.Context) error {
		handleTerminal(c.Response(), c.Request())
		return nil
	})
}
