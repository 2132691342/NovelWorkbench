// 本文件实现 JWT 令牌的签发与校验。

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
	PwdEpoch int64  `json:"pwd_epoch"` // 签发时的密码版本（改密后旧令牌立即失效）
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

// SignToken 为用户签发令牌，返回令牌串与有效期（秒）。pwdEpoch 为当前密码版本，改密后递增。
func (s *JWTSigner) SignToken(userULID string, pwdEpoch int64) (string, int64, error) {
	now := time.Now()
	claims := TokenClaims{
		UserULID: userULID,
		PwdEpoch: pwdEpoch,
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

// ParseToken 校验令牌并返回用户 ULID（不做密码版本比对，由中间件结合业务状态判断）。
func (s *JWTSigner) ParseToken(tokenString string) (string, error) {
	claims, err := s.ParseClaims(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserULID, nil
}

// ParseClaims 校验令牌并返回完整声明（中间件据此比对密码版本）。
func (s *JWTSigner) ParseClaims(tokenString string) (*TokenClaims, error) {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return s.secretKey, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
