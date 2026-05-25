package e2b

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type SignatureOpts struct {
	Path                string
	Operation           string // "read" or "write"
	User                string
	ExpirationInSeconds int
	EnvdAccessToken     string
}

type SignatureResult struct {
	Signature  string
	Expiration *int64
}

func GetSignature(opts SignatureOpts) (*SignatureResult, error) {
	if opts.EnvdAccessToken == "" {
		return &SignatureResult{
			Signature:  "",
			Expiration: nil,
		}, nil
	}

	expirationSeconds := opts.ExpirationInSeconds
	if expirationSeconds == 0 {
		expirationSeconds = 900
	}

	expiration := time.Now().Unix() + int64(expirationSeconds)

	message := fmt.Sprintf("%s:%s:%s:%d", opts.Operation, opts.Path, opts.User, expiration)

	mac := hmac.New(sha256.New, []byte(opts.EnvdAccessToken))
	mac.Write([]byte(message))
	signature := "v1_" + hex.EncodeToString(mac.Sum(nil))

	return &SignatureResult{
		Signature:  signature,
		Expiration: &expiration,
	}, nil
}
