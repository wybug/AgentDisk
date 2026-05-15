package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 18080, "Mock server port")
	defaultUserID := flag.String("userId", "user_mock_001", "Default user ID to return")
	flag.Parse()

	mux := http.NewServeMux()

	// /oauth2/authorize — auto-approve, redirect back with code
	mux.HandleFunc("/oauth2/authorize", func(w http.ResponseWriter, r *http.Request) {
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		prompt := r.URL.Query().Get("prompt")

		log.Printf("[mock] authorize: prompt=%s state=%s", prompt, state)

		if prompt == "none" {
			// Simulate: user not logged in
			// Change to test the happy path by commenting out the error return
		}

		code := "mock_code_for_" + *defaultUserID
		location := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, code, state)
		http.Redirect(w, r, location, http.StatusFound)
	})

	// /oauth2/token — return mock access token
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[mock] token exchange")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "mock_access_token_" + *defaultUserID,
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "mock_refresh_token",
		})
	})

	// /oauth2/userinfo — return mock user info
	mux.HandleFunc("/oauth2/userinfo", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[mock] userinfo")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"userId":   *defaultUserID,
			"userName": "Mock User",
		})
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mock OAuth2 server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
