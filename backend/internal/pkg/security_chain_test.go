// 安全链路的场景回归：受保护请求的放行与拒绝 + 供应商密钥的封存与解封。
// 两条链路在生产里才暴露问题（权限绕过、凭据解不开），因此合并到本文件按链路组织，
// 每条用例的步骤用「步骤N」标注，失败时可直接定位到断点。

package pkg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// permCheckerFunc 让测试用闭包实现 PermChecker。
type permCheckerFunc func(ctx context.Context, userULID string, permCode string) (bool, error)

// HasPerm 实现 PermChecker 接口。
func (f permCheckerFunc) HasPerm(ctx context.Context, userULID string, permCode string) (bool, error) {
	return f(ctx, userULID, permCode)
}

// auditRecorderFunc 让测试用闭包实现 AuditRecorder。
type auditRecorderFunc func(ctx context.Context, do *domain.AuditLogDO) error

// Record 实现 AuditRecorder 接口。
func (f auditRecorderFunc) Record(ctx context.Context, do *domain.AuditLogDO) error {
	return f(ctx, do)
}

// revokedTokenStore 测试用令牌吊销存储（仅标记一个被吊销的令牌）。
type revokedTokenStore struct {
	revoked string
}

// Revoke 实现 TokenStore 接口（测试不写黑名单）。
func (s revokedTokenStore) Revoke(context.Context, string, time.Duration) error { return nil }

// IsRevoked 实现 TokenStore 接口。
func (s revokedTokenStore) IsRevoked(_ context.Context, tokenString string) bool {
	return tokenString == s.revoked
}

// TestAccessControlChain 覆盖一次受保护请求的完整链路。
func TestAccessControlChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := NewJWTSigner("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, _, err := signer.SignToken("USR_TEST")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// 步骤1：放行链路。请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络。
	var auditAction, auditTargetType, auditTargetULID string
	recorder := auditRecorderFunc(func(_ context.Context, do *domain.AuditLogDO) error {
		auditAction, auditTargetType, auditTargetULID = do.ActionCode, do.TargetType, do.TargetULID
		return nil
	})
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(AuthMiddleware(signer, nil, func(context.Context, string) (bool, error) { return true, nil }, nil))
	router.Use(RequirePermMiddleware(permCheckerFunc(func(_ context.Context, _, permCode string) (bool, error) {
		return permCode == "system:user:manage", nil
	}), "system:user:manage"))
	router.Use(AuditMiddleware(recorder, nil))
	router.POST("/api/v1/admin/user/update", func(c *gin.Context) {
		Ok(c, gin.H{"user_ulid": c.GetString(ContextKeyUserULID)})
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/update",
		strings.NewReader(`{"user_ulid":"USR_TARGET"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Request-ID", "REQ_INJECTED")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		RespCode string `json:"resp_code"`
		TraceID  string `json:"trace_id"`
		Data     struct {
			UserULID string `json:"user_ulid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.RespCode != "SUCCESS" || envelope.TraceID != "REQ_INJECTED" || envelope.Data.UserULID != "USR_TEST" {
		t.Fatalf("包络或鉴权注入不符: %+v", envelope)
	}
	if auditAction != "admin/user/update" || auditTargetType != "user" || auditTargetULID != "USR_TARGET" {
		t.Fatalf("审计内容不符: %s/%s/%s", auditAction, auditTargetType, auditTargetULID)
	}

	// 步骤2：拒绝链路。缺失令牌 / 令牌已吊销 / 账号已禁用 → 401，权限不足 → 403。
	const revoked = "REVOKED_TOKEN"
	rejectCases := []struct {
		caseName     string
		authHeader   string
		revokedToken string
		userActive   bool
		permGranted  bool
		wantStatus   int
		wantRespCode string
	}{
		{"无令牌", "", "", true, true, http.StatusUnauthorized, "ERR_UNAUTHORIZED"},
		{"令牌已吊销", "Bearer " + revoked, revoked, true, true, http.StatusUnauthorized, "ERR_UNAUTHORIZED"},
		{"账号已禁用", "Bearer " + token, "", false, true, http.StatusUnauthorized, "ERR_UNAUTHORIZED"},
		{"权限不足", "Bearer " + token, "", true, false, http.StatusForbidden, "ERR_FORBIDDEN"},
	}
	for _, testCase := range rejectCases {
		rejectRouter := gin.New()
		rejectRouter.Use(RequestIDMiddleware())
		rejectRouter.Use(AuthMiddleware(signer,
			revokedTokenStore{revoked: testCase.revokedToken},
			func(context.Context, string) (bool, error) { return testCase.userActive, nil }, nil))
		rejectRouter.Use(RequirePermMiddleware(permCheckerFunc(func(context.Context, string, string) (bool, error) {
			return testCase.permGranted, nil
		}), "system:user:manage"))
		rejectRouter.GET("/api/v1/admin/user/list", func(c *gin.Context) { Ok(c, gin.H{"ok": true}) })

		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/user/list", nil)
		if testCase.authHeader != "" {
			request.Header.Set("Authorization", testCase.authHeader)
		}
		rejectResponse := httptest.NewRecorder()
		rejectRouter.ServeHTTP(rejectResponse, request)

		var rejectEnvelope struct {
			RespCode string `json:"resp_code"`
		}
		if err := json.Unmarshal(rejectResponse.Body.Bytes(), &rejectEnvelope); err != nil {
			t.Fatalf("%s: decode response: %v", testCase.caseName, err)
		}
		if rejectResponse.Code != testCase.wantStatus || rejectEnvelope.RespCode != testCase.wantRespCode {
			t.Fatalf("%s: status=%d resp_code=%q, want %d/%q", testCase.caseName,
				rejectResponse.Code, rejectEnvelope.RespCode, testCase.wantStatus, testCase.wantRespCode)
		}
	}
}

// TestSecretSealChain 覆盖供应商密钥的封存链路：封存 → 解封还原 → 密文篡改与材料变更的拒绝路径。
func TestSecretSealChain(t *testing.T) {
	const material = "jwt-secret-material"
	const plain = "sk-provider-key-0123456789"

	// 步骤1：封存与解封。同一明文两次封存得到不同密文（nonce 随机），但都能还原。
	sealed, err := SealSecret(plain, material)
	if err != nil {
		t.Fatalf("封存失败: %v", err)
	}
	opened, err := OpenSecret(sealed, material)
	if err != nil || opened != plain {
		t.Fatalf("解封应还原明文: %q, err=%v", opened, err)
	}
	sealed2, err := SealSecret(plain, material)
	if err != nil {
		t.Fatalf("二次封存失败: %v", err)
	}
	if sealed == sealed2 {
		t.Fatal("相同明文两次封存必须得到不同密文（nonce 随机）")
	}
	if opened2, err := OpenSecret(sealed2, material); err != nil || opened2 != plain {
		t.Fatalf("二次密文也应还原明文: %q, err=%v", opened2, err)
	}

	// 步骤2：密文被篡改或格式非法必须解不开（认证标签校验失败），不得 panic。
	tampered := []byte(sealed)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	for _, bad := range []string{string(tampered), "不是 base64!!!", "YWJj"} {
		if _, err := OpenSecret(bad, material); err != ErrSecretInvalid {
			t.Fatalf("非法密文应被拒绝: %q, err=%v", bad, err)
		}
	}

	// 步骤3：密钥材料变更后历史密文不可解；材料为空直接报错。
	if _, err := OpenSecret(sealed, "other-material"); err != ErrSecretInvalid {
		t.Fatalf("材料变更后应解不开: %v", err)
	}
	if _, err := SealSecret(plain, ""); err != ErrSecretEmpty {
		t.Fatalf("材料为空应报错: %v", err)
	}
}
