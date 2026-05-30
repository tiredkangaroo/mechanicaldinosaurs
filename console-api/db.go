package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tiredkangaroo/mechanicaldinosaurs/server"
)

// the functions exposed should make it easy to change between underlying db system
// just using json for now bc simple

type DB struct {
	file *os.File
}

type dbStructure struct {
	RemoteServers []server.RemoteServer `json:"remote_servers"`
}

var dbParsed dbStructure

// remote server stuff
func (db *DB) GetRemoteServers() ([]server.RemoteServer, error) {
	return dbParsed.RemoteServers, nil
}

func (db *DB) GetRemoteServer(serverName string) (*server.RemoteServer, error) {
	for i, server := range dbParsed.RemoteServers {
		if server.Name == serverName {
			return &dbParsed.RemoteServers[i], nil
		}
	}
	return nil, fmt.Errorf("remote server not found: %s", serverName)
}

func (db *DB) AddRemoteServer(server server.RemoteServer) error {
	dbParsed.RemoteServers = append(dbParsed.RemoteServers, server)
	if err := db.Write(); err != nil {
		return err
	}
	return nil
}

func (db *DB) UpdateRemoteServer(serverName string, updatedServer server.RemoteServer) error {
	for i, server := range dbParsed.RemoteServers {
		if server.Name == serverName {
			dbParsed.RemoteServers[i] = updatedServer
			if err := db.Write(); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("remote server not found: %s", serverName)
}

func (db *DB) RemoveRemoteServer(serverName string) error {
	for i, server := range dbParsed.RemoteServers {
		if server.Name == serverName {
			dbParsed.RemoteServers = append(dbParsed.RemoteServers[:i], dbParsed.RemoteServers[i+1:]...)
			if err := db.Write(); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("remote server not found: %s", serverName)
}

func (db *DB) Load() error {
	stat, err := db.file.Stat()
	if err != nil {
		return fmt.Errorf("stat db: %w", err)
	}
	if stat.Size() == 0 {
		dbParsed = dbStructure{}
		return nil
	}
	jsonBytes := make([]byte, stat.Size())
	if _, err := db.file.Read(jsonBytes); err != nil {
		return fmt.Errorf("read db: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &dbParsed); err != nil {
		return fmt.Errorf("unmarshal db: %w", err)
	}
	return nil
}

func (db *DB) Write() error {
	jsonBytes, err := json.Marshal(dbParsed)
	if err != nil {
		return fmt.Errorf("marshal db: %w", err)
	}
	// go to beginning of file and truncate before writing new data
	if _, err := db.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek db: %w", err)
	}
	if err := db.file.Truncate(0); err != nil {
		return fmt.Errorf("truncate db: %w", err)
	}

	if _, err := db.file.Write(jsonBytes); err != nil {
		return fmt.Errorf("write db: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	return db.file.Close()
}

func GetDB() (*DB, error) {
	dbFile := os.Getenv("DB_FILE")
	file, err := os.OpenFile(dbFile, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db := &DB{file: file}
	if err := db.Load(); err != nil {
		return nil, fmt.Errorf("load db: %w", err)
	}
	return db, nil
}
