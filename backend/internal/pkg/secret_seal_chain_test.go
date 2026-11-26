// 供应商密钥封存链路的回归：封存 → 解封还原 → 密文篡改与材料变更的拒绝路径。
// 密钥是模型调用凭据的唯一副本，密文解不开等于整个模型生态不可用；
// 篡改或材料变更时若仍能解出，等于凭据保护形同虚设，因此单独守住这条链路。

package pkg

import "testing"

// TestSecretSealChain 覆盖供应商密钥的封存链路。
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
