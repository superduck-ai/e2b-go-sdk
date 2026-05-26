package e2b

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

type signatureOpts struct {
	Path                string
	Operation           string // "read" or "write"
	User                string
	ExpirationInSeconds int
	EnvdAccessToken     string
}

type signatureResult struct {
	Signature  string
	Expiration *int64
}

func GetSignature(path string, operation string, user string, expirationInSeconds int, envdAccessToken string) (string, *int64, error) {
	opts := signatureOpts{
		Path:                path,
		Operation:           operation,
		User:                user,
		ExpirationInSeconds: expirationInSeconds,
		EnvdAccessToken:     envdAccessToken,
	}

	if opts.EnvdAccessToken == "" {
		return "", nil, fmt.Errorf("access token is not set and signature cannot be generated")
	}

	resolvedUser := opts.User
	raw := fmt.Sprintf("%s:%s:%s:%s", opts.Path, opts.Operation, resolvedUser, opts.EnvdAccessToken)

	var expiration *int64
	if opts.ExpirationInSeconds > 0 {
		exp := time.Now().Unix() + int64(opts.ExpirationInSeconds)
		expiration = &exp
		raw = fmt.Sprintf("%s:%d", raw, exp)
	}

	sum := sha256.Sum256([]byte(raw))
	signature := "v1_" + base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(sum[:])

	return signature, expiration, nil
}
