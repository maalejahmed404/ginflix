// jwt.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

type JSONWebKey struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Alg string   `json:"alg"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

var cachedJWKS *JWKS
var lastJWKSFetch time.Time

func tokenMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(auth, "Bearer ")
		claims, err := validateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Optional: Check audience, issuer, etc.
		if claims["aud"] != expectedAudience {
			http.Error(w, "Invalid audience", http.StatusUnauthorized)
			return
		}

		// Check if username matches client ID
		username, ok := claims["preferred_username"].(string)
		if !ok {
			http.Error(w, "Missing preferred_username claim", http.StatusUnauthorized)
			return
		}

		if username != kcClientId {
			http.Error(w, "Access denied: You are not authorized to access this resource", http.StatusForbidden)
			return
		}

		fmt.Println("Connection from User: ", claims["email"])

		next(w, r)
	}
}

func validateToken(tokenString string) (jwt.MapClaims, error) {
	jwks, err := getJWKS()
	if err != nil {
		return nil, err
	}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in header")
		}

		for _, key := range jwks.Keys {
			if key.Kid == kid {
				return parseRSAPublicKeyFromJWKS(key)
			}
		}
		return nil, errors.New("key not found")
	}

	token, err := jwt.Parse(tokenString, keyFunc)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("cannot parse claims")
	}

	return claims, nil
}

func getJWKS() (*JWKS, error) {
	if cachedJWKS != nil && time.Since(lastJWKSFetch) < 5*time.Minute {
		return cachedJWKS, nil
	}
	jwksURL := keycloakJwksURL
	if v := os.Getenv("KEYCLOAK_JWKS_URL"); v != "" {
		jwksURL = v
	}
	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	cachedJWKS = &jwks
	lastJWKSFetch = time.Now()
	return &jwks, nil
}
