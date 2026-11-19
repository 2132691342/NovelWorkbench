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

// TestRequestLifecycleChain 覆盖完整请求链路：请求 ID 透传 → 鉴权放行 → 权限判定 → 审计落库 → 响应包络。
// 这是一条横跨多个中间件的链路，任一环节断裂都会导致线上行为变化，因此保留为单一用例。
func TestRequestLifecycleChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := NewJWTSigner("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, _, err := signer.SignToken("USR_TEST")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

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
}

// TestAuthRejectionChain 覆盖鉴权拒绝链路：缺失令牌、吊销令牌、用户被禁用、权限不足分别落到正确业务码。
func TestAuthRejectionChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := NewJWTSigner("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, _, err := signer.SignToken("USR_TEST")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	revoked := "REVOKED_TOKEN"

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
		{caseName: "令牌已吊销", authHeader: "Bearer " + revoked, revokedToken: revoked, userActive: true, permGranted: true,
			wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "账号已禁用", authHeader: "Bearer " + token, userActive: false, permGranted: true,
			wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "权限不足", authHeader: "Bearer " + token, userActive: true, permGranted: false,
			wantStatus: http.StatusForbidden, wantRespCode: "ERR_FORBIDDEN"},
	}
	for _, testCase := range cases {
		t.Run(testCase.caseName, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestIDMiddleware())
			router.Use(AuthMiddleware(signer,
				revokedTokenStore{revoked: testCase.revokedToken},
				func(context.Context, string) (bool, error) { return testCase.userActive, nil }, nil))
			router.Use(RequirePermMiddleware(permCheckerFunc(func(context.Context, string, string) (bool, error) {
				return testCase.permGranted, nil
			}), "system:user:manage"))
			router.GET("/api/v1/admin/user/list", func(c *gin.Context) { Ok(c, gin.H{"ok": true}) })

			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/user/list", nil)
			if testCase.authHeader != "" {
				request.Header.Set("Authorization", testCase.authHeader)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
			var response struct {
				RespCode string `json:"resp_code"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.RespCode != testCase.wantRespCode {
				t.Fatalf("resp_code = %q, want %q", response.RespCode, testCase.wantRespCode)
			}
		})
	}
}

// TestMethodWhitelistChain 覆盖方法白名单链路：非 GET/POST 请求被拦截，且不进入业务处理器。
func TestMethodWhitelistChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MethodWhitelistMiddleware())
	reached := false
	router.DELETE("/api/v1/project/delete", func(c *gin.Context) {
		reached = true
		Ok(c, nil)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/v1/project/delete", nil))

	if reached {
		t.Fatal("DELETE 不应进入业务处理器")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	var response struct {
		RespCode string `json:"resp_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RespCode != "ERR_INVALID_PARAM" {
		t.Fatalf("resp_code = %q, want ERR_INVALID_PARAM", response.RespCode)
	}
}

// TestBusinessErrorMappingChain 覆盖错误映射链路：业务码到 HTTP 状态的映射唯一且未知码归 500。
func TestBusinessErrorMappingChain(t *testing.T) {
	cases := []struct {
		code       string
		wantStatus int
	}{
		{code: "SUCCESS", wantStatus: http.StatusOK},
		{code: "ERR_INVALID_PARAM", wantStatus: http.StatusBadRequest},
		{code: "ERR_UNAUTHORIZED", wantStatus: http.StatusUnauthorized},
		{code: "ERR_FORBIDDEN", wantStatus: http.StatusForbidden},
		{code: "ERR_NOT_FOUND", wantStatus: http.StatusNotFound},
		{code: "ERR_CONFLICT", wantStatus: http.StatusConflict},
		{code: "ERR_BIZ_FAILED", wantStatus: http.StatusBadRequest},
		{code: "ERR_AI_PROVIDER", wantStatus: http.StatusBadGateway},
		{code: "ERR_STORAGE", wantStatus: http.StatusServiceUnavailable},
		{code: "ERR_INTERNAL", wantStatus: http.StatusInternalServerError},
		{code: "ERR_UNKNOWN_CODE", wantStatus: http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		if got := HTTPStatusForCode(testCase.code); got != testCase.wantStatus {
			t.Fatalf("HTTPStatusForCode(%q) = %d, want %d", testCase.code, got, testCase.wantStatus)
		}
	}
}

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