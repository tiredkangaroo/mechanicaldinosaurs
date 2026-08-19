package main

import "os"

var MECHANICAL_DINOSAURS_DATA = os.Getenv("MECHANICAL_DINOSAURS_DATA")
var API_SECRET = os.Getenv("API_SECRET")
var API_PUBLIC_KEY = os.Getenv("API_PUBLIC_KEY") // alternative to the API_SECRET, a signed jwt can be used to auth to the api
var PORT = os.Getenv("PORT")
