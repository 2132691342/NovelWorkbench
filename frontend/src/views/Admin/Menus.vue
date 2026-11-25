<!-- 菜单管理页：树形维护菜单、路由、图标与权限码。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import {
    ElButton,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import { deleteMenu, listMenus } from '@/api/menu'
  import type { NWMenuVo } from '@/api/contracts'
  import { useRowAction } from '@/composables/useRowAction'
  import MenuFormDialog from './components/MenuFormDialog.vue'
  import { asRow } from '@/utils/table'

  const menuList = ref<NWMenuVo[]>([])
  const loading = ref(false)
  const keyword = ref('')
  const typeFilter = ref('')

  const formDialog = ref<InstanceType<typeof MenuFormDialog>>()
  const rowAction = useRowAction()

  const typeOptions = [
    { value: '', label: '全部类型' },
    { value: 'group', label: '分组' },
    { value: 'page', label: '页面' },
    { value: 'action', label: '动作' }
  ]

  const typeLabel = (typeCode: string) =>
    typeOptions.find((item) => item.value === typeCode)?.label ?? typeCode

  /** 按父子层级展开为有序行（同级按 sort_order 升序），并记录缩进深度 */
  const rows = computed(() => {
    const childrenMap = new Map<string, NWMenuVo[]>()
    menuList.value.forEach((item) => {
      const key = item.parent_ulid || ''
      const list = childrenMap.get(key) ?? []
      list.push(item)
      childrenMap.set(key, list)
    })
    childrenMap.forEach((list) => list.sort((a, b) => a.sort_order - b.sort_order))
    const ordered: Array<NWMenuVo & { depth: number }> = []
    const walk = (parentUlid: string, depth: number) => {
      for (const item of childrenMap.get(parentUlid) ?? []) {
        ordered.push({ ...item, depth })
        walk(item.menu_ulid, depth + 1)
      }
    }
    walk('', 0)
    // 父节点缺失的孤立菜单追加在末尾，避免数据异常时丢失
    const seen = new Set(ordered.map((item) => item.menu_ulid))
    menuList.value.forEach((item) => {
      if (!seen.has(item.menu_ulid)) ordered.push({ ...item, depth: 0 })
    })
    return ordered
  })

  const load = async () => {
    loading.value = true
    try {
      menuList.value = await listMenus({
        keyword_text: keyword.value || undefined,
        menu_type_code: typeFilter.value || undefined
      })
    } catch {
      menuList.value = []
    } finally {
      loading.value = false
    }
  }

  /** 删除菜单（含子节点时由后端拦截；行内加锁防连点） */
  const remove = async (row: NWMenuVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除菜单「${row.menu_name}」，其权限码将一并下线，确定吗？`,
        '删除菜单',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await rowAction.run(row.menu_ulid, async () => {
      await deleteMenu(row.menu_ulid)
      ElMessage.success('菜单已删除')
      await load()
    })
  }

  const handleSaved = () => {
    void load()
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
        placeholder="搜索名称 / 编码 / 权限码"
        clearable
        class="admin-menus__search"
        @keyup.enter="load()"
        @clear="load()"
      />
      <ElSelect v-model="typeFilter" class="admin-menus__type" @change="load()">
        <ElOption
          v-for="item in typeOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="load()">查询</ElButton>
      <ElButton type="primary" @click="formDialog?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-menus__btn-text">新建菜单</span>
      </ElButton>
      <div class="admin-menus__spacer" />
      <span class="nw-hint">共 {{ rows.length }} 项菜单</span>
    </div>

    <ElTable v-loading="loading" :data="rows" row-key="menu_ulid" class="admin-menus__table">
      <ElTableColumn label="菜单名称" min-width="200">
        <template #default="{ row }">
          <span :style="{ paddingLeft: `${row.depth * 16}px` }" class="admin-menus__name">
            <Icon
              :icon="row.menu_type_code === 'group' ? 'mdi:folder-outline' : 'mdi:file-document-outline'"
              :size="14"
            />
            {{ row.menu_name }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="上级" min-width="130">
        <template #default="{ row }">
          <span :class="row.parent_name ? '' : 'nw-hint'">{{ row.parent_name || '根级' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="menu_code" label="编码" min-width="150" show-overflow-tooltip />
      <ElTableColumn label="类型" width="80">
        <template #default="{ row }">
          <span class="nw-badge nw-badge--neutral">{{ typeLabel(row.menu_type_code) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="route_path" label="路由" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.route_path ? '' : 'nw-hint'">{{ row.route_path || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="icon_code" label="图标" width="150" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.icon_code ? '' : 'nw-hint'">{{ row.icon_code || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="perm_code" label="权限码" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">
          <code v-if="row.perm_code" class="admin-menus__code">{{ row.perm_code }}</code>
          <span v-else class="nw-hint">登录可见</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="展示" width="70" align="center">
        <template #default="{ row }">
          <Icon
            :icon="row.visible_flag ? 'mdi:check' : 'mdi:minus'"
            :size="15"
            :style="{ color: row.visible_flag ? 'var(--nw-success)' : 'var(--nw-text-muted)' }"
          />
        </template>
      </ElTableColumn>
      <ElTableColumn label="启用" width="70" align="center">
        <template #default="{ row }">
          <span class="nw-badge" :class="row.enabled_flag ? 'nw-badge--success' : 'nw-badge--danger'">
            {{ row.enabled_flag ? '是' : '否' }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="sort_order" label="排序" width="70" align="center" />
      <ElTableColumn label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDialog?.open(asRow<NWMenuVo>(row))">编辑</ElButton>
          <ElButton
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(asRow<NWMenuVo>(row).menu_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(asRow<NWMenuVo>(row))"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无菜单</span>
      </template>
    </ElTable>

    <MenuFormDialog ref="formDialog" :menu-list="menuList" @saved="handleSaved" />
  </div>
</template>

<style lang="less" scoped>
  .admin-menus {
    &__search {
      width: 240px;
    }

    &__type {
      width: 130px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      border-radius: var(--nw-radius-lg);
    }

    &__name {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__code {
      padding: 1px 6px;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--nw-text-secondary);
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
    }
  }
</style>
