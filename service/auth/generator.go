package auth

import (
	"crypto/rsa"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func loadPrivateKey(keyFile string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPrivateKeyFromPEM(b)
}

func makeToken(keyID string, keyFile string, claims jwt.MapClaims) (string, error) {
	privateKey, err := loadPrivateKey(keyFile)
	if err != nil {
		return "", err
	}
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims = claims
	token.Header["kid"] = keyID
	return token.SignedString(privateKey)
}

// GenerateAccessToken creates a JWT in string format with the expected audience,
// issuer, and claims to pass authentication checks.
func GenerateAccessToken(keyID string, privateKeyFile string, expiration time.Duration, audience, domain string, tenantID TenantID, userID UserID) (string, error) {
	dstr, err := url.Parse(domain)
	if err != nil {
		return "", fmt.Errorf("bad domain URL: %w", err)
	}
	return makeToken(keyID, privateKeyFile, jwt.MapClaims{
		"aud":         audience,
		"exp":         time.Now().Add(expiration).Unix(),
		"iss":         dstr.String() + "/",
		TenantIDClaim: string(tenantID),
		UserIDClaim:   string(userID),
	})
}
