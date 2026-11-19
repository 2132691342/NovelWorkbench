// 本文件实现供应商密钥的 AES-256-GCM 封存与解封。

package pkg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// 密钥封存错误常量（调用方据错误判断是否为配置损坏）。
var (
	ErrSecretEmpty   = errors.New("secret material is empty")   // 未提供封存材料
	ErrSecretInvalid = errors.New("sealed secret is malformed") // 密文格式非法
)

// SealSecret 以 AES-256-GCM 封存明文（密钥材料经 SHA-256 派生，输出 base64(nonce|密文)）。
// 仅用于模型供应商 API Key 一类可还原凭据；一旦材料变更，历史密文不可解。
func SealSecret(plainText string, keyMaterial string) (string, error) {
	aead, err := secretAEAD(keyMaterial)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// OpenSecret 解封 SealSecret 产出的密文。
func OpenSecret(sealedText string, keyMaterial string) (string, error) {
	aead, err := secretAEAD(keyMaterial)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(sealedText)
	if err != nil || len(raw) <= aead.NonceSize() {
		return "", ErrSecretInvalid
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return "", ErrSecretInvalid
	}
	return string(plain), nil
}

// secretAEAD 按密钥材料构造 AES-256-GCM。
func secretAEAD(keyMaterial string) (cipher.AEAD, error) {
	if keyMaterial == "" {
		return nil, ErrSecretEmpty
	}
	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
