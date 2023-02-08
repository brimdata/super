package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const (
	testAudience = "testaudience"
	testKeyID    = "testkey"
	testKeyFile  = "../testdata/auth-private-key"
	testJWKSFile = "../testdata/auth-public-jwks.json"
)

func testValidator(t *testing.T) *TokenValidator {
	v, err := NewTokenValidator(testAudience, "https://testdomain", testJWKSFile)
	require.NoError(t, err)
	return v
}

func genToken(t *testing.T, claims jwt.MapClaims) string {
	token, err := makeToken(testKeyID, testKeyFile, claims)
	require.NoError(t, err)
	return token
}

func TestValidate(t *testing.T) {
	expectedIdent := Identity{
		TenantID: "test_tenant_id",
		UserID:   "test_user_id",
	}
	token, err := GenerateAccessToken(testKeyID, testKeyFile, 1*time.Hour,
		testAudience, "https://testdomain", "test_tenant_id", "test_user_id")
	require.NoError(t, err)
	validator := testValidator(t)

	ident, err := validator.Validate(token)
	require.NoError(t, err)
	require.Equal(t, expectedIdent, ident)

	req, err := http.NewRequest("GET", "https://testdomain", nil)
	require.NoError(t, err)
	req.Header.Add("Authorization", "Bearer "+token)
	tokstr, ident, err := validator.ValidateRequest(req)
	require.NoError(t, err)
	require.Equal(t, expectedIdent, ident)
	require.Equal(t, token, tokstr)

	req, err = http.NewRequest("GET", "https://testdomain", nil)
	require.NoError(t, err)
	_, _, err = validator.ValidateRequest(req)
	require.Error(t, err)
}

func TestValidateNoTenantIDAndUserIDClaims(t *testing.T) {
	token, err := makeToken(testKeyID, testKeyFile, jwt.MapClaims{
		"aud": testAudience,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iss": "https://testdomain/",
	})
	require.NoError(t, err)
	ident, err := testValidator(t).Validate(token)
	require.NoError(t, err)
	require.Equal(t, Identity{AnonymousTenantID, AnonymousUserID}, ident)
}

func TestBadClaims(t *testing.T) {
	var cases = []struct {
		name  string
		token string
	}{
		{
			name: "missing audience",
			token: genToken(t, jwt.MapClaims{
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "invalid audience",
			token: genToken(t, jwt.MapClaims{
				"aud":         "foo",
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "missing expiration",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "expired expiration",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(-1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "missing issuer",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "invalid issuer",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "foo",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "empty user id",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   "",
			}),
		},
		{
			name: "anonymous user id",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "test_tenant_id",
				UserIDClaim:   AnonymousUserID,
			}),
		},
		{
			name: "empty tenant id",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: "",
				UserIDClaim:   "test_user_id",
			}),
		},
		{
			name: "anonymous tenant id",
			token: genToken(t, jwt.MapClaims{
				"aud":         testAudience,
				"exp":         time.Now().Add(1 * time.Hour).Unix(),
				"iss":         "https://testdomain/",
				TenantIDClaim: AnonymousTenantID,
				UserIDClaim:   "test_user_id",
			}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := testValidator(t)
			_, err := v.Validate(c.token)
			require.Error(t, err)
		})
	}
}

func TestKeyID(t *testing.T) {
	claims := jwt.MapClaims{
		"aud":         testAudience,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iss":         "https://testdomain/",
		TenantIDClaim: "test_tenant_id",
		UserIDClaim:   "test_user_id",
	}
	privateKey, err := loadPrivateKey(testKeyFile)
	require.NoError(t, err)
	v := testValidator(t)

	// Bad key id
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims = claims
	token.Header["kid"] = "foo"
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)
	_, err = v.Validate(tokenString)
	require.Error(t, err)

	// No key id
	token = jwt.New(jwt.SigningMethodRS256)
	token.Claims = claims
	tokenString, err = token.SignedString(privateKey)
	require.NoError(t, err)
	_, err = v.Validate(tokenString)
	require.Error(t, err)
}

func TestAudienceSlice(t *testing.T) {
	expectedIdent := Identity{
		TenantID: "test_tenant_id",
		UserID:   "test_user_id",
	}
	token := genToken(t, jwt.MapClaims{
		"aud":         []string{testAudience, "foobar"},
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iss":         "https://testdomain/",
		TenantIDClaim: "test_tenant_id",
		UserIDClaim:   "test_user_id",
	})
	validator := testValidator(t)
	ident, err := validator.Validate(token)
	require.NoError(t, err)
	require.Equal(t, expectedIdent, ident)

	token = genToken(t, jwt.MapClaims{
		"aud":         []string{"foo", "bar"},
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iss":         "https://testdomain/",
		TenantIDClaim: "test_tenant_id",
		UserIDClaim:   "test_user_id",
	})
	_, err = validator.Validate(token)
	require.Error(t, err)
}
