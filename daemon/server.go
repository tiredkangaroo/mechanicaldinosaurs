package main

import (
	"encoding/json"
	"log"
)

func main() {
	serverinfo, err := GetServerInfo()
	if err != nil {
		log.Fatal(err)
	}
	serverinfoJSON, err := json.Marshal(serverinfo)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("server info: %s", serverinfoJSON)
}
