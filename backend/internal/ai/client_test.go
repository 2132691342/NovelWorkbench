// 本文件校验模型网关的 HTTP 客户端构造：流式读 body 不能被整体超时切断。

package ai

import (
	"net/http"
	"testing"
	"time"
)

// TestStreamHTTPClientTimeoutPolicy 断言流式客户端不设整体超时，首包超时取自供应商配置。
func TestStreamHTTPClientTimeoutPolicy(t *testing.T) {
	client := newStreamHTTPClient(45 * time.Second)
	if client.Timeout != 0 {
		t.Fatalf("流式客户端不得设置整体超时（会切断长流），实际 %v", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("应使用 *http.Transport，实际 %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("首包超时应取供应商配置，实际 %v", transport.ResponseHeaderTimeout)
	}
	if transport.MaxIdleConnsPerHost <= 0 {
		t.Fatalf("连接池必须按并发上限收敛，实际 %d", transport.MaxIdleConnsPerHost)
	}
}
