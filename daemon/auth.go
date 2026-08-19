package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

type Auth struct {
	Secret string
	locked bool // lock is permanent for now
}

var jwtPublicKey, _ = jwt.ParseECPublicKeyFromPEM([]byte(API_PUBLIC_KEY))

func (a *Auth) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if a.locked {
			log := fmt.Sprintf("attempted use during lock from ip %s, forwarded: %s", c.Request().RemoteAddr, c.RealIP())
			writeLog(log) // this could lowk just like resource max me if sb spammed ts
			return c.String(http.StatusLocked, "the daemon has been locked.")
		}
		authorization := c.Request().Header.Get("Authorization")
		if authorization == "" {
			return c.String(http.StatusUnauthorized, "no authorization")
		}
		valid := false
		if c.Request().Header.Get("X-Authorization-Method") == "jwt" {
			if jwtPublicKey == nil {
				return c.String(http.StatusInternalServerError, "jwt public key not configured")
			}
			tokenString := authorization[len("Bearer "):]
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return jwtPublicKey, nil
			})
			valid = err == nil && token.Valid
		} else {
			valid = "Bearer "+a.Secret == authorization
		}
		if !valid {
			a.locked = true
			writeLog(fmt.Sprintf("locked by ip %s, forwarded: %s. authorization given: %s", c.Request().RemoteAddr, c.RealIP(), authorization)) // again another possible resource max if sb gave a really long  auth header
			return c.String(http.StatusLocked, "the daemon has been locked.")                                                                   // won't tell what attempt locked the system if multiple attempts go at once and are distributed and unable to be synchronized
		}
		return next(c)
	}
}

func writeLog(msg string) {
	f, err := os.OpenFile("md-daemon-auth.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("open md-daemon-auth.log file", "error", err)
	}
	defer f.Close()

	if _, err := f.Write([]byte(msg)); err != nil {
		slog.Error("writing to md-daemon-auth.log failed", "error", err)
	}
}
