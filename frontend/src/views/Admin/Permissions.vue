<!-- 权限目录页：只读展示由菜单汇总出的权限分组。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElCard, ElEmpty, ElInput } from 'element-plus'
  import { fetchPermCatalog } from '@/api/menu'
  import type { NWPermGroupVo, NWPermItemVo } from '@/api/contracts'

  const groups = ref<NWPermGroupVo[]>([])
  const loading = ref(false)
  const keyword = ref('')

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
            (item.menu_name ?? '').toLowerCase().includes(keywordText)
        )
      }))
      .filter((group) => group.perm_list.length > 0)
  })

  const totalCount = computed(() =>
    groups.value.reduce((sum, group) => sum + group.perm_list.length, 0)
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

  const ownerLabel = (item: NWPermItemVo) =>
    item.menu_name || (item.menu_code ? item.menu_code : '动作权限')

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="keyword"
        placeholder="搜索权限码 / 名称 / 归属菜单"
        clearable
        class="admin-permissions__search"
      />
      <div class="admin-permissions__spacer" />
      <span class="nw-hint">共 {{ totalCount }} 项权限 · {{ groups.length }} 个分组</span>
    </div>

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
              <span v-if="item.builtin_flag" class="nw-badge nw-badge--neutral">内置</span>
            </div>
            <p class="perm-item__name">{{ item.perm_name }}</p>
            <p class="perm-item__desc">{{ item.perm_desc || '暂无说明' }}</p>
            <p class="perm-item__owner">
              <Icon icon="mdi:menu" :size="12" />
              {{ ownerLabel(item) }}
            </p>
          </li>
        </ul>
      </ElCard>

      <ElEmpty v-if="!loading && filteredGroups.length === 0" description="没有匹配的权限项" />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .admin-permissions {
    &__search {
      width: 280px;
    }

    &__spacer {
      flex: 1;
    }

    &__groups {
      display: grid;
      gap: 14px;
      align-items: start;
      min-height: 160px;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
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
      color: var(--nw-primary);
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
      color: var(--nw-text-secondary);
    }

    &__owner {
      display: flex;
      gap: 4px;
      align-items: center;
      margin-top: 6px;
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }
  }
</style>
