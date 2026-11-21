// 供应商密钥封存链路的回归测试：封存 → 解封还原 → 密文篡改与材料变更的拒绝路径。
// 这里用 AES-256-GCM，篡改密文必须被认证标签拒绝；材料变更必须让历史密文不可解。
// 一旦失效，平台会拿着解不开的凭据去调模型网关，表现为「连接性测试一直失败」，因此整体守住。

package pkg

import (
	"strings"
	"testing"
)

// TestSecretSealChain 覆盖密钥封存的完整链路：封存 → 解封 → 密文不可读 → 材料变更后失效。
func TestSecretSealChain(t *testing.T) {
	// 环节1：封存与解封。同一明文两次封存得到不同密文（随机 nonce），但都能还原。
	const material = "jwt-secret-material"
	const plain = "sk-provider-key-0123456789"

	sealed, err := SealSecret(plain, material)
	if err != nil {
		t.Fatalf("封存失败: %v", err)
	}
	if sealed == plain || !strings.HasPrefix(sealed, "") {
		t.Fatalf("密文不得等于明文: %q", sealed)
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
	opened2, err := OpenSecret(sealed2, material)
	if err != nil || opened2 != plain {
		t.Fatalf("二次密文也应还原明文: %q, err=%v", opened2, err)
	}

	// 环节2：密文被篡改必须解不开（认证标签校验失败）。
	tampered := []byte(sealed)
	if tampered[len(tampered)-1] == 'A' {
		tampered[len(tampered)-1] = 'B'
	} else {
		tampered[len(tampered)-1] = 'A'
	}
	if _, err := OpenSecret(string(tampered), material); err != ErrSecretInvalid {
		t.Fatalf("篡改密文应被拒绝: %v", err)
	}

	// 环节3：密文格式非法（非法 base64、长度不足）必须报格式错而不是 panic。
	for _, bad := range []string{"不是 base64!!!", "YWJj"} {
		if _, err := OpenSecret(bad, material); err != ErrSecretInvalid {
			t.Fatalf("非法密文 %q 应被拒绝: %v", bad, err)
		}
	}

	// 环节4：密钥材料变更后历史密文不可解；材料为空直接报错。
	if _, err := OpenSecret(sealed, "other-material"); err != ErrSecretInvalid {
		t.Fatalf("材料变更后应解不开: %v", err)
	}
	if _, err := SealSecret(plain, ""); err != ErrSecretEmpty {
		t.Fatalf("材料为空应报错: %v", err)
	}
	if _, err := OpenSecret(sealed, ""); err != ErrSecretEmpty {
		t.Fatalf("材料为空应报错: %v", err)
	}
}
