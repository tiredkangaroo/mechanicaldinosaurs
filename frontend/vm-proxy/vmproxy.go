package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

var PORT = os.Getenv("PORT")       // port to listen on
var DB_FILE = os.Getenv("DB_FILE") // path to sqlite db file

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", DB_FILE)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// sqlite does not support concurrent write; limit to 1 connection
	db.SetMaxOpenConns(1)

	listener, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		log.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	log.Printf("listening on port %s", PORT)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}

		log.Printf("accepted connection from %s", conn.RemoteAddr())
		go handleConnection(conn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	var vmName, hostport, secretKey, createdAt string

	// FIX: Corrected SQL syntax (FROM must come before WHERE)
	query := `
SELECT s.vm_name, s.created_at, m.hostport, m.secret_key
FROM console_vmsession s
JOIN console_machine m ON s.machine_id = m.name
WHERE s.claimed = FALSE
ORDER BY s.created_at DESC
LIMIT 1
`
	err := db.QueryRow(query).Scan(&vmName, &createdAt, &hostport, &secretKey)
	if err == sql.ErrNoRows {
		log.Printf("unauthorized: no active sessions found in database")
		return
	} else if err != nil {
		log.Printf("database error: %v", err)
		return
	}

	clientAddr := clientConn.RemoteAddr().String()

	// modify the session to mark it as claimed
	claimQuery := `UPDATE console_vmsession SET claimed = TRUE, claimed_by = ? WHERE vm_name = ?`
	_, err = db.Exec(claimQuery, clientAddr, vmName)
	if err != nil {
		log.Printf("failed to claim session %s by %s: %v", vmName, clientAddr, err)
		return
	}

	// delete session after closing
	defer func() {
		deleteQuery := `DELETE FROM console_vmsession WHERE vm_name = ?`
		if _, err := db.Exec(deleteQuery, vmName); err != nil {
			log.Printf("failed to delete session %s after closing: %v", vmName, err)
		} else {
			log.Printf("successfully deleted session %s from database", vmName)
		}
	}()

	log.Printf("forwarding connection from %s to latest session: %s (created at %s)", clientAddr, vmName, createdAt)

	daemonConn, err := net.DialTimeout("tcp", hostport, 5*time.Second)
	if err != nil {
		log.Printf("failed to connect to backend %s: %v", hostport, err)
		return
	}
	defer daemonConn.Close()

	// form the proxy req
	reqURL := fmt.Sprintf("http://%s/api/vms/%s/proxy", hostport, vmName)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Printf("failed to create request: %v", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+secretKey)
	if err := req.Write(daemonConn); err != nil {
		log.Printf("failed to write request: %v", err)
		return
	}

	errChan := make(chan error, 2)

	// client to daemon
	go func() {
		_, err := io.Copy(daemonConn, clientConn)
		errChan <- err
	}()

	// daemon to client
	go func() {
		_, err := io.Copy(clientConn, daemonConn)
		errChan <- err
	}()

	if err := <-errChan; err != nil {
		log.Printf("connection closed: %v", err)
	}
}
