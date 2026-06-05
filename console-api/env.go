package main

import "os"

type Config struct {
	STARTING_USER_NAME        string
	STARTING_USER_TOTP_SECRET string
}

var DefaultConfig Config

func init() {
	DefaultConfig.STARTING_USER_NAME = os.Getenv("STARTING_USER_NAME")
	DefaultConfig.STARTING_USER_TOTP_SECRET = os.Getenv("STARTING_TOTP_SECRET")
}
