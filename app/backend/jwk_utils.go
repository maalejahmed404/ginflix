// jwks_utils.go
package main

import (
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
)

func parseRSAPublicKeyFromJWKS(jwk JSONWebKey) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, errors.New("failed to decode n: " + err.Error())
	}
	eb, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, errors.New("failed to decode e: " + err.Error())
	}

	e := 0
	for _, b := range eb {
		e = e<<8 + int(b)
	}

	pubKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nb),
		E: e,
	}
	return pubKey, nil
}
