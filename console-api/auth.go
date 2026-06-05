package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	database "github.com/tiredkangaroo/mechanicaldinosaurs/console-api/db"
)

var JWT_SECRET = []byte("supersecretkey") // NOTE: yeah prob env var/hashicorp vault
var INACTIVITY_TIMEOUT = 5 * time.Minute  // NOTE: user setting later
var TOTP_OPTS = totp.ValidateOpts{
	Period:    15,
	Skew:      1,
	Digits:    otp.DigitsEight,
	Algorithm: otp.AlgorithmSHA512,
}

func checkTOTP(ctx context.Context, queries *database.Queries, username, givenTOTP string) error {
	user, err := queries.GetUser(ctx, username)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if !user.Active {
		return fmt.Errorf("user is not active")
	}
	if ok, err := totp.ValidateCustom(givenTOTP, user.TotpSecret, time.Now(), TOTP_OPTS); err != nil || !ok {
		return fmt.Errorf("invalid login credentials")
	}
	return nil
}

func issueJWT(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(INACTIVITY_TIMEOUT)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(JWT_SECRET)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signedToken, nil
}

func createAuthMiddleware(db *database.Queries) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/api/login" || (c.Path() == "/api/users" && c.Request().Method == http.MethodGet) {
				return next(c) // skip auth for login endpoint obv
			}

			// get auth token
			authorizationCookie, err := c.Cookie("auth")
			if err != nil {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			authorization := authorizationCookie.Value
			if authorization == "" {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}

			// validate token
			claims := new(jwt.RegisteredClaims)
			_, err = jwt.ParseWithClaims(authorization, claims, func(token *jwt.Token) (interface{}, error) {
				return JWT_SECRET, nil
			})
			if err != nil {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now()) {
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			if claims.IssuedAt == nil || claims.IssuedAt.Time.After(time.Now()) { // idk why this would happen by why not
				slog.Error("invalid token claims", "claims", claims)
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			if claims.Subject == "" { // no one you're claiming to be?
				slog.Error("invalid token claims", "claims", claims)
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}
			// check user in db (and check if active)
			user, err := db.GetUser(c.Request().Context(), claims.Subject)
			if err != nil || !user.Active {
				slog.Error("invalid user in token claims", "claims", claims, "error", err, "active", user.Active)
				return c.JSON(401, map[string]string{"error": "unauthorized"})
			}

			// new token to bump expiration
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(INACTIVITY_TIMEOUT)) // refresh expiration on each request
			newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signedToken, err := newToken.SignedString(JWT_SECRET)
			if err != nil {
				slog.Error("sign new token", "error", err)
				return c.JSON(500, map[string]string{"error": "internal server error"})
			}
			c.SetCookie(&http.Cookie{
				Name:    "auth",
				Value:   signedToken,
				Expires: claims.ExpiresAt.Time,
				// Secure: true, // NOTE: enable when we have https
				HttpOnly: true,
			})

			c.Set("user", user)
			return next(c)
		}
	}
}

func addAuthRoutes(api *echo.Group, db *database.Queries) {
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
		if err := db.AddUser(c.Request().Context(), createUserRequest); err != nil {
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
}
