package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/brimdata/super/pkg/fs"
	"github.com/brimdata/super/service/srverr"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
)

const (
	// These are the namespaced custom claims we expect on any JWT
	// access token.
	TenantIDClaim = "https://lake.brimdata.io/tenant_id"
	UserIDClaim   = "https://lake.brimdata.io/user_id"
)

type TokenValidator struct {
	expectedAudience string
	expectedIssuer   string
	keyGetter        jwt.Keyfunc
}

func NewTokenValidator(audience, domain, jwksPath string) (*TokenValidator, error) {
	domainURL, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("bad auth.domain URL: %w", err)
	}
	keys, err := loadPublicKeys(jwksPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load JWKS file: %w", err)
	}
	// Auth0 issuer is always the domain URL with trailing "/".
	// https://auth0.com/docs/tokens/access-tokens/get-access-tokens#custom-domains-and-the-management-api
	expectedIssuer := domainURL.String() + "/"
	keyGetter := func(token *jwt.Token) (any, error) {
		tokenKeyID, _ := token.Header["kid"].(string)
		key, ok := keys[tokenKeyID]
		if !ok {
			return token, errors.New("unknown token key id")
		}
		return key, nil
	}
	return &TokenValidator{
		expectedAudience: audience,
		expectedIssuer:   expectedIssuer,
		keyGetter:        keyGetter,
	}, nil
}

func (v *TokenValidator) ValidateRequest(r *http.Request) (string, Identity, error) {
	token, err := request.AuthorizationHeaderExtractor.ExtractToken(r)
	if err != nil {
		return "", Identity{}, srverr.ErrNoCredentials(err)
	}
	ident, err := v.Validate(token)
	if err != nil {
		return "", Identity{}, err
	}
	return token, ident, nil
}

func (v *TokenValidator) Validate(token string) (Identity, error) {
	if token == "" {
		return Identity{}, srverr.ErrNoCredentials()
	}
	parsed, err := jwt.Parse(token, v.keyGetter)
	if err != nil || !parsed.Valid {
		return Identity{}, srverr.ErrNoCredentials("invalid token")
	}
	if parsed.Header["alg"] != jwt.SigningMethodRS256.Alg() {
		return Identity{}, srverr.ErrNoCredentials("invalid signing method")
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if !claims.VerifyAudience(v.expectedAudience, true) {
		return Identity{}, srverr.ErrNoCredentials("invalid audience")
	}
	// jwt-go verifies any expiry claim, but will not fail if the expiry claim
	// is missing. The call here with req=true ensures that the claim is both
	// present and valid.
	if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
		return Identity{}, srverr.ErrNoCredentials("invalid expiration")
	}
	if !claims.VerifyIssuer(v.expectedIssuer, true) {
		return Identity{}, srverr.ErrNoCredentials("invalid issuer")
	}
	ident := Identity{AnonymousTenantID, AnonymousUserID}
	if v, ok := claims[TenantIDClaim]; ok {
		s, _ := v.(string)
		if s == "" || TenantID(s) == AnonymousTenantID {
			return Identity{}, srverr.ErrNoCredentials("invalid tenant ID")
		}
		ident.TenantID = TenantID(s)
	}
	if v, ok := claims[UserIDClaim]; ok {
		s, _ := v.(string)
		if s == "" || UserID(s) == AnonymousUserID {
			return Identity{}, srverr.ErrNoCredentials("invalid tenant ID")
		}
		ident.UserID = UserID(s)
	}
	return ident, nil
}

// jwks matches the format of a JSON Web Key Set file:
// https://auth0.com/docs/tokens/json-web-tokens/json-web-key-sets
type jwks struct {
	Keys []struct {
		Kty string   `json:"kty"`
		Kid string   `json:"kid"`
		Use string   `json:"use"`
		N   string   `json:"n"`
		E   string   `json:"e"`
		X5c []string `json:"x5c"`
	} `json:"keys"`
}

func loadPublicKeys(jwkspath string) (map[string]*rsa.PublicKey, error) {
	var jwks jwks
	if err := fs.UnmarshalJSONFile(jwkspath, &jwks); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if len(jwk.X5c) == 0 {
			continue
		}
		cert := "-----BEGIN CERTIFICATE-----\n" + jwk.X5c[0] + "\n-----END CERTIFICATE-----"
		public, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
		if err != nil {
			return nil, err
		}
		keys[jwk.Kid] = public
	}
	return keys, nil
}
