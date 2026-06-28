package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	target, err := url.Parse("http://localhost:8000")
	if err != nil {
		panic("unreachable")
	}

	// wowie there's a built in reverse proxy
	// i love u golang
	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Host = target.Host
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Real-IP", req.RemoteAddr)

		log.Printf("proxied request: %s %s -> %s", req.Method, req.URL.Path, target.String())
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("proxy error: %v", err)
		http.Error(rw, "proxy error", http.StatusBadGateway)
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: proxy,
		TLSConfig: &tls.Config{
			NextProtos: []string{"http/1.1"},
		},
	}

	if err := server.ListenAndServeTLS("server.crt", "server.key"); err != nil {
		log.Fatalf("failed: %v", err)
	}
}
