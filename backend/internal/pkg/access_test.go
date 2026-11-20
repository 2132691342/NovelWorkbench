// 本文件覆盖访问控制链路（请求 ID → 鉴权 → 权限判定 → 审计 → 响应包络）与密钥封存链路。

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

// TestAccessControlChain 覆盖请求访问控制链路：
// 放行路径为 请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；
// 拒绝路径为 缺失令牌 / 令牌已吊销 / 账号已禁用 → 401，权限不足 → 403。
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

	// ---- 放行链路 ----
	type auditRecord struct {
		UserULID   string
		ActionCode string
		TargetType string
		TargetULID string
	}
	var audits []auditRecord
	recorder := auditRecorderFunc(func(_ context.Context, do *domain.AuditLogDO) error {
		audits = append(audits, auditRecord{
			UserULID: do.UserULID, ActionCode: do.ActionCode,
			TargetType: do.TargetType, TargetULID: do.TargetULID,
		})
		return nil
	})

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(AuthMiddleware(signer, nil, func(context.Context, string) (bool, error) { return true, nil }, nil))
	router.Use(RequirePermMiddleware(permCheckerFunc(func(_ context.Context, userULID, permCode string) (bool, error) {
		return permCode == "system:user:manage", nil
	}), "system:user:manage"))
	router.Use(AuditMiddleware(recorder, nil))
	router.POST("/api/v1/admin/user/update", func(c *gin.Context) {
		Ok(c, gin.H{"user_ulid": c.GetString(ContextKeyUserULID)})
	})

	body := strings.NewReader(`{"user_ulid":"USR_TARGET"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user/update", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Request-ID", "REQ_INJECTED")
	recorder2 := httptest.NewRecorder()
	router.ServeHTTP(recorder2, request)

	if recorder2.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder2.Code, recorder2.Body.String())
	}
	var response struct {
		RespCode string `json:"resp_code"`
		TraceID  string `json:"trace_id"`
		Data     struct {
			UserULID string `json:"user_ulid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder2.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RespCode != "SUCCESS" || response.TraceID != "REQ_INJECTED" {
		t.Fatalf("包络不符: %+v", response)
	}
	if response.Data.UserULID != "USR_TEST" {
		t.Fatalf("鉴权未注入用户: %+v", response.Data)
	}
	if len(audits) != 1 {
		t.Fatalf("审计记录数 = %d, want 1", len(audits))
	}
	if audits[0].ActionCode != "admin/user/update" || audits[0].TargetType != "user" || audits[0].TargetULID != "USR_TARGET" {
		t.Fatalf("审计内容不符: %+v", audits[0])
	}

	// ---- 拒绝链路 ----
	rejected := "REVOKED_TOKEN"
	cases := []struct {
		caseName     string
		authHeader   string
		revokedToken string
		userActive   bool
		permGranted  bool
		wantStatus   int
		wantRespCode string
	}{
		{caseName: "无令牌", authHeader: "", userActive: true, permGranted: true,
			wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "令牌已吊销", authHeader: "Bearer " + rejected, revokedToken: rejected, userActive: true, permGranted: true,
			wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "账号已禁用", authHeader: "Bearer " + token, userActive: false, permGranted: true,
			wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "权限不足", authHeader: "Bearer " + token, userActive: true, permGranted: false,
			wantStatus: http.StatusForbidden, wantRespCode: "ERR_FORBIDDEN"},
	}
	for _, testCase := range cases {
		t.Run(testCase.caseName, func(t *testing.T) {
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
			recorder3 := httptest.NewRecorder()
			rejectRouter.ServeHTTP(recorder3, request)

			if recorder3.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder3.Code, testCase.wantStatus)
			}
			var rejectResponse struct {
				RespCode string `json:"resp_code"`
			}
			if err := json.Unmarshal(recorder3.Body.Bytes(), &rejectResponse); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if rejectResponse.RespCode != testCase.wantRespCode {
				t.Fatalf("resp_code = %q, want %q", rejectResponse.RespCode, testCase.wantRespCode)
			}
		})
	}
}

// TestSecretSealOpenChain 覆盖供应商密钥封存链路：同材料可解、换材料/坏密文/空材料必须失败、nonce 随机。
// 密钥材料变更即历史密文全部不可解，因此这几条失败路径必须一起守住。
func TestSecretSealOpenChain(t *testing.T) {
	sealed, err := SealSecret("sk-test-1234567890", "jwt-secret-material")
	if err != nil {
		t.Fatalf("SealSecret error = %v", err)
	}
	if sealed == "" || sealed == "sk-test-1234567890" {
		t.Fatalf("封存结果不应为空或明文: %q", sealed)
	}
	plain, err := OpenSecret(sealed, "jwt-secret-material")
	if err != nil {
		t.Fatalf("OpenSecret error = %v", err)
	}
	if plain != "sk-test-1234567890" {
		t.Fatalf("解封结果 = %q, want sk-test-1234567890", plain)
	}
	if _, err := OpenSecret(sealed, "another-material"); err == nil {
		t.Fatal("换用其他材料解封必须失败")
	}
	if _, err := OpenSecret("not-base64-text", "jwt-secret-material"); err == nil {
		t.Fatal("非法密文必须失败")
	}
	if _, err := SealSecret("x", ""); err == nil {
		t.Fatal("空材料必须失败")
	}

	second, err := SealSecret("sk-test-1234567890", "jwt-secret-material")
	if err != nil {
		t.Fatalf("SealSecret error = %v", err)
	}
	if second == sealed {
		t.Fatal("两次封存结果必须不同（nonce 随机）")
	}
}
