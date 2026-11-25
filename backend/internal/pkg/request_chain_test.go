// 请求管线链路的回归：请求 ID → 鉴权 → 权限判定 → 审计落库（含响应体）→ 响应包络，
// 以及写请求幂等与供应商密钥封存。三者都是「生产里才暴露」的链路，任一环断裂都会静默放过错误，
// 因此集中在本文件按链路组织，用例内的步骤用「步骤N」标注，失败时直接定位到断点。

package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// permStub 是 PermChecker 的测试替身：按权限码判定是否放行。
type permStub map[string]bool

// HasPerm 实现 PermChecker 接口。
func (s permStub) HasPerm(_ context.Context, _ string, permCode string) (bool, error) {
	return s[permCode], nil
}

// auditStub 是 AuditRecorder 的测试替身：记住最后一条审计记录。
type auditStub struct{ last *domain.AuditLogDO }

// Record 实现 AuditRecorder 接口。
func (s *auditStub) Record(_ context.Context, do *domain.AuditLogDO) error {
	s.last = do
	return nil
}

// revokeStub 是 TokenStore 的测试替身：标记一个被吊销的令牌。
type revokeStub struct{ revoked string }

// Revoke 实现 TokenStore 接口（测试不写黑名单）。
func (s revokeStub) Revoke(context.Context, string, time.Duration) error { return nil }

// IsRevoked 实现 TokenStore 接口。
func (s revokeStub) IsRevoked(_ context.Context, tokenString string) bool {
	return tokenString == s.revoked
}

// TestRequestPipelineChain 覆盖一次受保护请求从进入到出参的完整管线。
func TestRequestPipelineChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := NewJWTSigner("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, _, err := signer.SignToken("USR_TEST", 0)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// 步骤1：放行链路。请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络。
	recorder := &auditStub{}
	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(AuthMiddleware(signer, nil, func(context.Context, string) (bool, int64, error) { return true, 0, nil }, nil))
	router.Use(RequirePermMiddleware(permStub{"system:user:manage": true}, "system:user:manage"))
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

	// 步骤2：审计必须同时留下请求与响应。缺响应体就等于「只知道发生了什么，不知道结果如何」。
	if recorder.last == nil || recorder.last.ActionCode != "admin/user/update" ||
		recorder.last.TargetType != "user" || recorder.last.TargetULID != "USR_TARGET" {
		t.Fatalf("审计内容不符: %+v", recorder.last)
	}
	if !strings.Contains(recorder.last.RequestBody, "USR_TARGET") {
		t.Fatalf("审计应记录请求体: %q", recorder.last.RequestBody)
	}
	if !strings.Contains(recorder.last.RespBody, "SUCCESS") {
		t.Fatalf("审计应记录响应体: %q", recorder.last.RespBody)
	}

	// 步骤3：拒绝链路。缺失令牌 / 令牌已吊销 / 账号已禁用 → 401，权限不足 → 403。
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
		{"令牌已吊销", "Bearer REVOKED_TOKEN", "REVOKED_TOKEN", true, true, http.StatusUnauthorized, "ERR_UNAUTHORIZED"},
		{"账号已禁用", "Bearer " + token, "", false, true, http.StatusUnauthorized, "ERR_UNAUTHORIZED"},
		{"权限不足", "Bearer " + token, "", true, false, http.StatusForbidden, "ERR_FORBIDDEN"},
	}
	for _, testCase := range rejectCases {
		rejectRouter := gin.New()
		rejectRouter.Use(RequestIDMiddleware())
		rejectRouter.Use(AuthMiddleware(signer,
			revokeStub{revoked: testCase.revokedToken},
			func(context.Context, string) (bool, int64, error) { return testCase.userActive, 0, nil }, nil))
		rejectRouter.Use(RequirePermMiddleware(permStub{"system:user:manage": testCase.permGranted}, "system:user:manage"))
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

// memoryIdempotencyStore 内存版幂等存储（Redis 的测试替身，语义与 SET NX / SET XX 对齐）。
type memoryIdempotencyStore struct {
	mutex sync.Mutex        // 并发保护
	data  map[string]string // 键 → 记录 JSON
}

// Claim 实现 IdempotencyStore：仅当键不存在时写入。
func (s *memoryIdempotencyStore) Claim(_ context.Context, key string, record *IdempotencyRecord,
	_ int) (*IdempotencyRecord, bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if raw, ok := s.data[key]; ok {
		existing, err := UnmarshalIdempotencyRecord(raw)
		if err != nil {
			return nil, false, err
		}
		return existing, true, nil
	}
	raw, err := MarshalIdempotencyRecord(record)
	if err != nil {
		return nil, false, err
	}
	s.data[key] = raw
	return nil, false, nil
}

// Save 实现 IdempotencyStore：覆盖写已完成的首响应。
func (s *memoryIdempotencyStore) Save(_ context.Context, key string, record *IdempotencyRecord, _ int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	raw, err := MarshalIdempotencyRecord(record)
	if err != nil {
		return err
	}
	s.data[key] = raw
	return nil
}

// Release 实现 IdempotencyStore：删除键。
func (s *memoryIdempotencyStore) Release(_ context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
	return nil
}

// TestIdempotencyChain 覆盖一次带幂等键的写请求链路：只执行一次 → 同键重放 → 同键异体拒绝 → 故障可重试。
func TestIdempotencyChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &memoryIdempotencyStore{data: map[string]string{}}
	executeCount := 0
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(ContextKeyUserULID, "USR_TEST")
		c.Next()
	})
	engine.Use(IdempotencyMiddleware(store, 60, nil))
	engine.POST("/api/v1/thing/save", func(c *gin.Context) {
		executeCount++
		c.JSON(http.StatusOK, gin.H{"resp_code": "SUCCESS", "resp_message": "ok",
			"data": gin.H{"execute_count": executeCount}})
	})

	perform := func(key string, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/thing/save", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(IdempotencyHeader, key)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		return recorder
	}

	// 步骤1：首请求正常执行一次，不带重放标记。
	first := perform("KEY_1", `{"thing_name":"a"}`)
	if executeCount != 1 || first.Header().Get(IdempotencyReplayHeader) != "" {
		t.Fatalf("首请求应执行一次且无重放标记: count=%d header=%q",
			executeCount, first.Header().Get(IdempotencyReplayHeader))
	}

	// 步骤2：同键同体重放，不重复执行，响应体与首响应逐字节一致。
	second := perform("KEY_1", `{"thing_name":"a"}`)
	if executeCount != 1 {
		t.Fatalf("重放不应再次执行，实际执行 %d 次", executeCount)
	}
	if second.Header().Get(IdempotencyReplayHeader) != "true" {
		t.Fatal("重放响应应带 Idempotent-Replay 标记")
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("重放响应体应与首响应一致\nfirst=%s\nsecond=%s", first.Body.String(), second.Body.String())
	}

	// 步骤3：同键不同体必须拒绝，避免把两次不同动作误认为一次。
	conflict := perform("KEY_1", `{"thing_name":"b"}`)
	var envelope struct {
		RespCode string `json:"resp_code"`
	}
	if err := json.Unmarshal(conflict.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析冲突响应失败: %v", err)
	}
	if envelope.RespCode == "SUCCESS" || executeCount != 1 {
		t.Fatalf("同键不同体应拒绝且不执行，实际 resp_code=%s execute=%d", envelope.RespCode, executeCount)
	}

	// 步骤4：服务端故障不缓存，客户端可用同键重试并真正执行。
	failing := gin.New()
	failing.Use(func(c *gin.Context) { c.Set(ContextKeyUserULID, "USR_TEST"); c.Next() })
	failing.Use(IdempotencyMiddleware(store, 60, nil))
	failCount := 0
	recoverFlag := false
	failing.POST("/api/v1/thing/save", func(c *gin.Context) {
		failCount++
		if !recoverFlag {
			c.JSON(http.StatusInternalServerError, gin.H{"resp_code": "ERR_INTERNAL", "resp_message": "boom"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resp_code": "SUCCESS", "resp_message": "ok", "data": nil})
	})
	runFail := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/thing/save", bytes.NewBufferString(`{"thing_name":"a"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(IdempotencyHeader, "KEY_2")
		recorder := httptest.NewRecorder()
		failing.ServeHTTP(recorder, req)
		return recorder
	}
	runFail()
	recoverFlag = true
	recovered := runFail()
	if failCount != 2 || !strings.Contains(recovered.Body.String(), "SUCCESS") {
		t.Fatalf("故障后同键重试应再次执行并成功: count=%d body=%s", failCount, recovered.Body.String())
	}

	// 步骤5：未带幂等键的请求不受影响（不写缓存、不拦截）。
	plain := gin.New()
	plain.Use(IdempotencyMiddleware(store, 60, nil))
	plainCount := 0
	plain.POST("/api/v1/thing/save", func(c *gin.Context) {
		plainCount++
		c.JSON(http.StatusOK, gin.H{"resp_code": "SUCCESS"})
	})
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/thing/save", bytes.NewBufferString(`{}`))
		plain.ServeHTTP(httptest.NewRecorder(), req)
	}
	if plainCount != 2 {
		t.Fatalf("未带幂等键的请求应每次都执行，实际 %d 次", plainCount)
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
