// 菜单种子的完整性回归：每个登记的组件路径都必须有同名前端页面文件，
// 否则「菜单即路由」会在点击时落到 404（例如顶栏「个人中心」入口）。
package service

import (
	"os"
	"path/filepath"

	"testing"
)

// TestMenuSeedComponentFiles 校验种子里的 component_path 都能在 frontend/src 下找到页面文件。
func TestMenuSeedComponentFiles(t *testing.T) {
	srcRoot := filepath.Join("..", "..", "..", "frontend", "src")
	if _, err := os.Stat(srcRoot); err != nil {
		t.Skipf("跳过：未找到前端源码目录 %s", srcRoot)
	}
	for _, seed := range menuSeedList {
		if seed.ComponentPath == "" {
			continue
		}
		basePath := filepath.Join(srcRoot, filepath.FromSlash(seed.ComponentPath))
		if fileExistsWithExt(basePath) {
			continue
		}
		t.Errorf("菜单 %s 的组件路径 %s 在前端不存在", seed.MenuCode, seed.ComponentPath)
	}
}

// TestMenuSeedUniqueCodes 校验菜单编码与路由路径唯一（前端以编码生成路由名）。
func TestMenuSeedUniqueCodes(t *testing.T) {
	seenCode := map[string]bool{}
	seenRoute := map[string]bool{}
	for _, seed := range menuSeedList {
		if seenCode[seed.MenuCode] {
			t.Errorf("菜单编码重复：%s", seed.MenuCode)
		}
		seenCode[seed.MenuCode] = true
		if seed.RoutePath == "" {
			continue
		}
		if seenRoute[seed.RoutePath] {
			t.Errorf("菜单路由重复：%s", seed.RoutePath)
		}
		seenRoute[seed.RoutePath] = true
	}
}

// TestProfileMenuRegistered 固定「个人中心」入口：顶栏用户菜单直接跳 /profile，缺登记即 404。
func TestProfileMenuRegistered(t *testing.T) {
	for _, seed := range menuSeedList {
		if seed.MenuCode != "profile" {
			continue
		}
		if seed.RoutePath != "/profile" || seed.ComponentPath != "views/Profile/index" || !seed.HiddenFlag {
			t.Fatalf("个人中心菜单定义不符合预期：%+v", seed)
		}
		return
	}
	t.Fatal("缺少 profile 菜单登记")
}

// fileExistsWithExt 判定页面文件（.vue 或 .tsx）是否存在。
func fileExistsWithExt(basePath string) bool {
	for _, ext := range []string{".vue", ".tsx"} {
		if info, err := os.Stat(basePath + ext); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
