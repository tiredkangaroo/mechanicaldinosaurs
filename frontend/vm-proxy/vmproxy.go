package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var PORT = os.Getenv("PORT")
var DISCONNECT_PORT = os.Getenv("DISCONNECT_PORT")
var DISCONNECT_SECRET = os.Getenv("DISCONNECT_SECRET")
var DB_FILE = os.Getenv("DB_FILE")
var db *sql.DB

var sessionIDsToErrChan = make(map[string]chan error)
var sessionIDsToErrChanLock = &sync.Mutex{}

func main() {
	if len(DISCONNECT_SECRET) != 128 {
		log.Fatalf("DISCONNECT_SECRET must be 128 characters long")
	}

	var err error
	db, err = sql.Open("sqlite", DB_FILE)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	db.SetMaxOpenConns(1)

	listener, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		log.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	disconnectListener, err := net.Listen("tcp", ":"+DISCONNECT_PORT)
	if err != nil {
		log.Fatalf("Failed to start disconnect listener: %v", err)
	}
	defer disconnectListener.Close()

	go func() {
		for {
			conn, err := disconnectListener.Accept()
			if err != nil {
				log.Printf("failed to accept disconnect connection: %v", err)
				continue
			}
			go handleDisconnect(conn)
		}
	}()

	log.Printf("listening on port %s", PORT)
	log.Printf("disconnect listener on port %s", DISCONNECT_PORT)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	var vmName, hostport, secretKey, createdAt, sessionID string

	query := `
		SELECT s.session_id, s.vm_name, s.created_at, m.hostport, m.secret_key
		FROM console_vmsession s
		JOIN console_machine m ON s.machine_id = m.name
		WHERE s.claimed = FALSE
		ORDER BY s.created_at DESC
		LIMIT 1`

	err := db.QueryRow(query).Scan(&sessionID, &vmName, &createdAt, &hostport, &secretKey)
	if err == sql.ErrNoRows {
		log.Printf("unauthorized: no active sessions found in database")
		return
	} else if err != nil {
		log.Printf("database error: %v", err)
		return
	}

	clientAddr := clientConn.RemoteAddr().String()

	claimQuery := `UPDATE console_vmsession SET claimed = TRUE, claimed_by = ? WHERE session_id = ?`
	_, err = db.Exec(claimQuery, clientAddr, sessionID)
	if err != nil {
		log.Printf("failed to claim session %s by %s: %v", sessionID, clientAddr, err)
		return
	}

	defer func() {
		deleteQuery := `DELETE FROM console_vmsession WHERE session_id = ?`
		if _, err := db.Exec(deleteQuery, sessionID); err != nil {
			log.Printf("failed to delete session %s after closing: %v", sessionID, err)
		} else {
			log.Printf("successfully deleted session %s from database", sessionID)
		}
	}()

	log.Printf("forwarding connection from %s to %s (created at %s, session id: %s)", clientAddr, vmName, createdAt, sessionID)

	daemonConn, err := net.DialTimeout("tcp", hostport, 5*time.Second)
	if err != nil {
		log.Printf("failed to connect to backend %s: %v", hostport, err)
		return
	}
	defer daemonConn.Close()

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

	// buffer size of 3: 2 for the io.Copy goroutines, 1 for the potential disconnect trigger
	errChan := make(chan error, 3)

	sessionIDsToErrChanLock.Lock()
	sessionIDsToErrChan[sessionID] = errChan
	sessionIDsToErrChanLock.Unlock()

	// clean up map entry when leaving this function scope
	defer func() {
		sessionIDsToErrChanLock.Lock()
		delete(sessionIDsToErrChan, sessionID)
		sessionIDsToErrChanLock.Unlock()
	}()

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
		log.Printf("connection closed for session %s: %v", sessionID, err)
	}
}

func handleDisconnect(conn net.Conn) {
	defer conn.Close()

	secret := make([]byte, 128)
	if _, err := io.ReadFull(conn, secret); err != nil {
		log.Printf("failed to read complete secret: %v", err)
		return
	}

	if string(secret) != DISCONNECT_SECRET {
		log.Printf("unauthorized disconnect attempt from %s", conn.RemoteAddr())
		return
	}

	sessionID := make([]byte, 36)
	if _, err := io.ReadFull(conn, sessionID); err != nil {
		log.Printf("failed to read complete session ID: %v", err)
		return
	}

	sessionIDStr := string(sessionID)
	log.Printf("received disconnect request for session ID: %s", sessionIDStr)

	sessionIDsToErrChanLock.Lock()
	errChan, exists := sessionIDsToErrChan[sessionIDStr]
	if exists {
		// remove it immediately so double-disconnect requests don't duplicate efforts
		delete(sessionIDsToErrChan, sessionIDStr)
	}
	sessionIDsToErrChanLock.Unlock()

	if !exists {
		log.Printf("no active connection found for session ID: %s", sessionIDStr)
		return
	}

	// this safe send works because errChan is buffered to 3 items
	errChan <- fmt.Errorf("disconnect requested remotely")
	log.Printf("successfully sent disconnect signal for session ID: %s", sessionIDStr)
}
