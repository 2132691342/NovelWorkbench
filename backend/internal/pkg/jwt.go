package pkg

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// JWT 相关错误。
var (
	ErrTokenInvalid = errors.New("token invalid") // 令牌无效或过期
)

// TokenClaims JWT 声明体。
type TokenClaims struct {
	UserULID string `json:"user_ulid"` // 用户 ULID
	jwt.RegisteredClaims
}

// JWTSigner 签发与校验 JWT 的组件（构造时注入密钥与有效期）。
type JWTSigner struct {
	secretKey []byte        // 签名密钥
	expireTTL time.Duration // 令牌有效期
}

// NewJWTSigner 构造 JWTSigner；secret 为空时返回错误，启动期 fail fast。
func NewJWTSigner(secret string, expireTTL time.Duration) (*JWTSigner, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}
	return &JWTSigner{secretKey: []byte(secret), expireTTL: expireTTL}, nil
}

// SignToken 为用户签发令牌，返回令牌串与有效期（秒）。
func (s *JWTSigner) SignToken(userULID string) (string, int64, error) {
	now := time.Now()
	claims := TokenClaims{
		UserULID: userULID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expireTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "NovelWorkbench",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", 0, WrapBizError(domain.RespCodeInternalError, "签发令牌失败", err)
	}
	return signed, int64(s.expireTTL / time.Second), nil
}

// ParseToken 校验令牌并返回用户 ULID。
func (s *JWTSigner) ParseToken(tokenString string) (string, error) {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return s.secretKey, nil
	})
	if err != nil || !token.Valid {
		return "", ErrTokenInvalid
	}
	return claims.UserULID, nil
}
