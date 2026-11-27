<!-- 权限目录页：按模块分组展示权限码、来源与引用角色，支持搜索与一键复制。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElButton, ElCard, ElEmpty, ElInput, ElMessage, ElTooltip } from 'element-plus'
  import { fetchPermCatalog } from '@/api/menu'
  import type { NWPermGroupVo, NWPermItemVo } from '@/api/contracts'

  const groups = ref<NWPermGroupVo[]>([])
  const loading = ref(false)
  const keyword = ref('')
  const copiedCode = ref('')

  /** 按关键字过滤权限项，过滤后为空的组不展示 */
  const filteredGroups = computed(() => {
    const keywordText = keyword.value.trim().toLowerCase()
    if (!keywordText) return groups.value
    return groups.value
      .map((group) => ({
        ...group,
        perm_list: group.perm_list.filter(
          (item) =>
            item.perm_code.toLowerCase().includes(keywordText) ||
            item.perm_name.toLowerCase().includes(keywordText) ||
            (item.perm_desc ?? '').toLowerCase().includes(keywordText) ||
            (item.menu_name ?? '').toLowerCase().includes(keywordText) ||
            (item.role_name_list ?? []).some((name) => name.toLowerCase().includes(keywordText))
        )
      }))
      .filter((group) => group.perm_list.length > 0)
  })

  const totalCount = computed(() =>
    groups.value.reduce((sum, group) => sum + group.perm_list.length, 0)
  )

  const visibleCount = computed(() =>
    filteredGroups.value.reduce((sum, group) => sum + group.perm_list.length, 0)
  )

  const load = async () => {
    loading.value = true
    try {
      groups.value = await fetchPermCatalog()
    } catch {
      groups.value = []
    } finally {
      loading.value = false
    }
  }

  /** 来源说明：页面权限标「菜单」，动作节点标「动作」 */
  const sourceLabel = (item: NWPermItemVo) => {
    if (!item.menu_code) return '系统内置'
    return item.menu_name || item.menu_code
  }

  /** 复制权限码（剪贴板不可用时退化为提示手抄，不静默失败） */
  const copyCode = async (permCode: string) => {
    try {
      await navigator.clipboard.writeText(permCode)
      copiedCode.value = permCode
      ElMessage.success(`已复制 ${permCode}`)
      window.setTimeout(() => {
        if (copiedCode.value === permCode) copiedCode.value = ''
      }, 1500)
    } catch {
      ElMessage.warning('浏览器未授权剪贴板，请手动选择复制')
    }
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="keyword"
        placeholder="搜索权限码 / 名称 / 归属菜单 / 角色"
        clearable
        class="admin-permissions__search"
      />
      <ElButton :loading="loading" @click="load()">
        <Icon icon="mdi:refresh" :size="15" />
      </ElButton>
      <div class="admin-permissions__spacer" />
      <span class="nw-hint">
        {{ keyword ? `匹配 ${visibleCount} / ${totalCount} 项` : `共 ${totalCount} 项权限` }} ·
        {{ filteredGroups.length }} 个分组
      </span>
    </div>

    <p class="nw-hint admin-permissions__intro">
      权限码由菜单管理登记：新增一行带权限码的菜单或动作，目录与角色勾选项会自动出现；
      删除或停用菜单会同步从所有角色中剔除该权限码。
    </p>

    <div v-loading="loading" class="admin-permissions__groups">
      <ElCard
        v-for="group in filteredGroups"
        :key="group.group_code"
        shadow="never"
        class="perm-group"
      >
        <template #header>
          <div class="perm-group__head">
            <span class="perm-group__name">{{ group.group_name }}</span>
            <span class="nw-badge nw-badge--neutral">{{ group.perm_list.length }}</span>
          </div>
        </template>
        <ul class="perm-group__list">
          <li v-for="item in group.perm_list" :key="item.perm_code" class="perm-item">
            <div class="perm-item__head">
              <code class="perm-item__code">{{ item.perm_code }}</code>
              <ElTooltip content="复制权限码" placement="top">
                <ElButton
                  link
                  size="small"
                  class="perm-item__copy"
                  @click="copyCode(item.perm_code)"
                >
                  <Icon
                    :icon="copiedCode === item.perm_code ? 'mdi:check' : 'mdi:content-copy'"
                    :size="13"
                  />
                </ElButton>
              </ElTooltip>
              <span v-if="item.builtin_flag" class="nw-badge nw-badge--neutral">内置</span>
            </div>
            <p class="perm-item__name">{{ item.perm_name }}</p>
            <p class="perm-item__desc">{{ item.perm_desc || '暂无说明' }}</p>
            <div class="perm-item__meta">
              <span class="perm-item__owner">
                <Icon icon="mdi:menu" :size="12" />
                {{ sourceLabel(item) }}
              </span>
              <span v-if="item.role_name_list?.length" class="perm-item__roles">
                <span
                  v-for="roleName in item.role_name_list"
                  :key="roleName"
                  class="nw-badge nw-badge--neutral"
                >
                  {{ roleName }}
                </span>
              </span>
              <span v-else class="nw-hint">暂无角色引用</span>
            </div>
          </li>
        </ul>
      </ElCard>

      <ElEmpty
        v-if="!loading && filteredGroups.length === 0"
        :description="keyword ? '没有匹配的权限项' : '暂无权限登记'"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .admin-permissions {
    &__search {
      width: 300px;
    }

    &__spacer {
      flex: 1;
    }

    &__intro {
      margin-top: -8px;
    }

    &__groups {
      display: grid;
      gap: 14px;
      align-items: start;
      min-height: 160px;
      grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    }
  }

  .perm-group {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    &__name {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: 10px;
      list-style: none;
    }
  }

  .perm-item {
    padding: 10px 12px;
    background: var(--nw-surface-subtle);
    border-radius: var(--nw-radius-md);

    &__head {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    &__code {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 12px;
      color: var(--el-color-primary);
    }

    &__copy {
      padding: 0;
      height: 18px;
    }

    &__name {
      margin-top: 6px;
      font-size: 13px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__desc {
      margin-top: 2px;
      font-size: 12px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
    }

    &__meta {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      margin-top: 8px;
    }

    &__owner {
      display: flex;
      gap: 4px;
      align-items: center;
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__roles {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }
  }
</style>
