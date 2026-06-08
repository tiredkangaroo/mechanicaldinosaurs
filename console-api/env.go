package main

import "os"

type Config struct {
	// starting user
	STARTING_USER_NAME        string
	STARTING_USER_TOTP_SECRET string
	// tls
	CERT_PATH string
	KEY_PATH  string
}

var DefaultConfig Config

func init() {
	DefaultConfig.STARTING_USER_NAME = os.Getenv("STARTING_USER_NAME")
	DefaultConfig.STARTING_USER_TOTP_SECRET = os.Getenv("STARTING_TOTP_SECRET")
	DefaultConfig.CERT_PATH = os.Getenv("CERT_PATH")
	DefaultConfig.KEY_PATH = os.Getenv("KEY_PATH")
}
