package pkg

import "testing"

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
