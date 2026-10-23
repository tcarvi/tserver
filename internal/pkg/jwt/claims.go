package jwtutil

import (
	"errors"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Permissions []string `json:"permissions"`
	Scope       string   `json:"scope"`
}

type Principal struct {
	Subject     string
	Permissions map[string]struct{}
}

func ParseBearerToken(raw string, secret []byte, issuer string) (Principal, error) {
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) {
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil || token == nil || !token.Valid || claims.Subject == "" {
		return Principal{}, errors.New("invalid bearer token")
	}

	permissions := make(map[string]struct{}, len(claims.Permissions))
	for _, permission := range claims.Permissions {
		permissions[permission] = struct{}{}
	}
	for _, permission := range strings.Fields(claims.Scope) {
		permissions[permission] = struct{}{}
	}
	return Principal{Subject: claims.Subject, Permissions: permissions}, nil
}
