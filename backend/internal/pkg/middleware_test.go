package pkg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestAuthMiddlewareUserState 验证禁用/删除用户后旧令牌立即失效，活跃用户正常放行。
func TestAuthMiddlewareUserState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	signer, err := NewJWTSigner("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, _, err := signer.SignToken("USR_TEST")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	cases := []struct {
		caseName     string
		active       bool
		checkErr     error
		wantStatus   int
		wantRespCode string
	}{
		{caseName: "active", active: true, wantStatus: http.StatusOK, wantRespCode: "SUCCESS"},
		{caseName: "disabled", active: false, wantStatus: http.StatusUnauthorized, wantRespCode: "ERR_UNAUTHORIZED"},
		{caseName: "check_failed", checkErr: context.DeadlineExceeded, wantStatus: http.StatusInternalServerError, wantRespCode: "ERR_INTERNAL"},
	}
	for _, testCase := range cases {
		t.Run(testCase.caseName, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestIDMiddleware())
			router.Use(AuthMiddleware(signer, nil, func(ctx context.Context, userULID string) (bool, error) {
				return testCase.active, testCase.checkErr
			}, nil))
			router.GET("/protected", func(c *gin.Context) { Ok(c, gin.H{"ok": true}) })

			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
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
