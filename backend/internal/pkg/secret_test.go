package pkg

import "testing"

// TestSealSecretRoundTrip 覆盖密钥封存链路：同材料可解、错材料与坏密文必须失败。
func TestSealSecretRoundTrip(t *testing.T) {
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
}

// TestSealSecretRandomized 同一明文两次封存结果必须不同（随机 nonce）。
func TestSealSecretRandomized(t *testing.T) {
	first, err := SealSecret("same-plain", "material")
	if err != nil {
		t.Fatalf("SealSecret error = %v", err)
	}
	second, err := SealSecret("same-plain", "material")
	if err != nil {
		t.Fatalf("SealSecret error = %v", err)
	}
	if first == second {
		t.Fatal("两次封存结果必须不同（nonce 随机）")
	}
}
