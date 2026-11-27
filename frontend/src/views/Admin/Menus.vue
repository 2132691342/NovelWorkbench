<!-- 菜单管理页：树形层级、可见/启用即时开关、按类型联动的编辑抽屉。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import {
    ElButton,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTable,
    ElTableColumn,
    ElTooltip
  } from 'element-plus'
  import { deleteMenu, listMenus, saveMenu } from '@/api/menu'
  import type { NWMenuVo } from '@/api/contracts'
  import { useRowAction } from '@/composables/useRowAction'
  import MenuFormDrawer from './components/MenuFormDrawer.vue'
  import { asRow } from '@/utils/table'

  type MenuRow = NWMenuVo & { depth: number; child_count: number }

  const menuList = ref<NWMenuVo[]>([])
  const loading = ref(false)
  const keyword = ref('')
  const typeFilter = ref('')
  const expandedKeys = ref<string[]>([])
  const togglingKey = ref('')

  const formDrawer = ref<InstanceType<typeof MenuFormDrawer>>()
  const rowAction = useRowAction()

  const typeOptions = [
    { value: '', label: '全部类型' },
    { value: 'group', label: '分组' },
    { value: 'page', label: '页面' },
    { value: 'action', label: '动作' }
  ]

  const typeLabel = (typeCode: string) =>
    typeOptions.find((item) => item.value === typeCode)?.label ?? typeCode

  const scopeLabel = (scopeCode: string) => (scopeCode === 'project' ? '项目内' : '全局')

  const typeBadge = (typeCode: string) =>
    typeCode === 'group' ? 'neutral' : typeCode === 'action' ? 'warning' : 'success'

  /** 父 → 子映射（同级按 sort_order 升序），供折叠计算与上级下拉复用 */
  const childrenMap = computed(() => {
    const map = new Map<string, NWMenuVo[]>()
    menuList.value.forEach((item) => {
      const key = item.parent_ulid || ''
      const list = map.get(key) ?? []
      list.push(item)
      map.set(key, list)
    })
    map.forEach((list) => list.sort((a, b) => a.sort_order - b.sort_order))
    return map
  })

  /** 折叠后的可见行：按展开状态剪枝，并记录缩进深度与下级数量 */
  const rows = computed<MenuRow[]>(() => {
    const expanded = new Set(expandedKeys.value)
    const ordered: MenuRow[] = []
    const walk = (parentUlid: string, depth: number) => {
      for (const item of childrenMap.value.get(parentUlid) ?? []) {
        const childCount = childrenMap.value.get(item.menu_ulid)?.length ?? 0
        ordered.push({ ...item, depth, child_count: childCount })
        if (childCount > 0 && expanded.has(item.menu_ulid)) {
          walk(item.menu_ulid, depth + 1)
        }
      }
    }
    walk('', 0)
    // 父节点缺失的孤立菜单追加在末尾，避免数据异常时整行丢失
    const seen = new Set(ordered.map((item) => item.menu_ulid))
    menuList.value.forEach((item) => {
      if (!seen.has(item.menu_ulid)) ordered.push({ ...item, depth: 0, child_count: 0 })
    })
    return ordered
  })

  const hasCollapsible = computed(() => menuList.value.some((item) => item.parent_ulid === '' && (childrenMap.value.get(item.menu_ulid)?.length ?? 0) > 0))

  const load = async () => {
    loading.value = true
    try {
      menuList.value = await listMenus({
        keyword_text: keyword.value || undefined,
        menu_type_code: typeFilter.value || undefined
      })
      // 首次加载展开全部含下级的节点，避免用户看到一排根节点却不知道下面有什么
      if (expandedKeys.value.length === 0) {
        expandedKeys.value = menuList.value
          .filter((item) => (childrenMap.value.get(item.menu_ulid)?.length ?? 0) > 0)
          .map((item) => item.menu_ulid)
      }
    } catch {
      menuList.value = []
    } finally {
      loading.value = false
    }
  }

  const toggleExpand = (row: MenuRow) => {
    if (row.child_count === 0) return
    expandedKeys.value = expandedKeys.value.includes(row.menu_ulid)
      ? expandedKeys.value.filter((key) => key !== row.menu_ulid)
      : [...expandedKeys.value, row.menu_ulid]
  }

  const expandAll = () => {
    expandedKeys.value = menuList.value
      .filter((item) => (childrenMap.value.get(item.menu_ulid)?.length ?? 0) > 0)
      .map((item) => item.menu_ulid)
  }

  const collapseAll = () => {
    expandedKeys.value = []
  }

  /** 即时保存可见/启用开关：失败时重新拉列表回滚，避免界面与库不一致 */
  const toggleFlag = async (row: MenuRow, field: 'visible_flag' | 'enabled_flag', value: boolean) => {
    const flagKey = `${row.menu_ulid}:${field}`
    togglingKey.value = flagKey
    try {
      await saveMenu({
        menu_ulid: row.menu_ulid,
        parent_ulid: row.parent_ulid || undefined,
        menu_code: row.menu_code,
        menu_name: row.menu_name,
        menu_type_code: row.menu_type_code,
        scope_code: row.scope_code,
        route_path: row.route_path,
        component_path: row.component_path,
        icon_code: row.icon_code,
        perm_code: row.perm_code,
        visible_flag: field === 'visible_flag' ? value : row.visible_flag,
        enabled_flag: field === 'enabled_flag' ? value : row.enabled_flag,
        sort_order: row.sort_order,
        remark_text: row.remark_text
      })
      ElMessage.success(field === 'visible_flag'
        ? (value ? '已在导航中展示' : '已从导航隐藏')
        : (value ? '菜单已启用' : '菜单已停用，其权限码将从角色授权中剔除'))
      await load()
    } catch {
      await load()
    } finally {
      togglingKey.value = ''
    }
  }

  /** 删除菜单（有下级时按钮已禁用；后端仍会兜底校验） */
  const remove = async (row: MenuRow) => {
    try {
      await ElMessageBox.confirm(
        `将删除菜单「${row.menu_name}」，其权限码将一并下线并从所有角色中剔除。此操作不可恢复。`,
        '删除菜单',
        { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
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
      <ElButton type="primary" @click="formDrawer?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-menus__btn-text">新建菜单</span>
      </ElButton>
      <div class="admin-menus__spacer" />
      <ElButton v-if="hasCollapsible" size="small" text @click="expandAll()">全部展开</ElButton>
      <ElButton v-if="hasCollapsible" size="small" text @click="collapseAll()">全部折叠</ElButton>
      <span class="nw-hint">共 {{ menuList.length }} 项菜单</span>
    </div>

    <ElTable v-loading="loading" :data="rows" row-key="menu_ulid" class="admin-menus__table">
      <ElTableColumn label="名称" min-width="240">
        <template #default="{ row }">
          <span class="admin-menus__name" :style="{ paddingLeft: `${row.depth * 18}px` }">
            <ElButton
              v-if="row.child_count > 0"
              link
              size="small"
              class="admin-menus__caret"
              @click="toggleExpand(asRow<MenuRow>(row))"
            >
              <Icon
                :icon="expandedKeys.includes(row.menu_ulid) ? 'mdi:minus' : 'mdi:plus'"
                :size="12"
              />
            </ElButton>
            <span v-else class="admin-menus__caret-placeholder" />
            <Icon
              :icon="row.menu_type_code === 'group' ? 'mdi:folder-outline' : row.menu_type_code === 'action' ? 'mdi:key-outline' : 'mdi:file-document-outline'"
              :size="14"
            />
            {{ row.menu_name }}
            <span v-if="row.child_count > 0" class="nw-hint">{{ row.child_count }}</span>
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="menu_code" label="编码" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <code class="admin-menus__code">{{ row.menu_code }}</code>
        </template>
      </ElTableColumn>
      <ElTableColumn label="类型" width="80">
        <template #default="{ row }">
          <span class="nw-badge" :class="`nw-badge--${typeBadge(row.menu_type_code)}`">
            {{ typeLabel(row.menu_type_code) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="作用域" width="90">
        <template #default="{ row }">
          <span :class="row.scope_code === 'project' ? '' : 'nw-hint'">
            {{ scopeLabel(row.scope_code) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="route_path" label="路由" min-width="150" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.route_path ? '' : 'nw-hint'">{{ row.route_path || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="权限码" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">
          <code v-if="row.perm_code" class="admin-menus__code">{{ row.perm_code }}</code>
          <span v-else class="nw-hint">登录可见</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="sort_order" label="排序" width="70" align="center" />
      <ElTableColumn label="展示" width="80" align="center">
        <template #default="{ row }">
          <ElSwitch
            size="small"
            :model-value="row.visible_flag"
            :loading="togglingKey === `${row.menu_ulid}:visible_flag`"
            :disabled="Boolean(togglingKey)"
            @change="toggleFlag(asRow<MenuRow>(row), 'visible_flag', Boolean($event))"
          />
        </template>
      </ElTableColumn>
      <ElTableColumn label="启用" width="80" align="center">
        <template #default="{ row }">
          <ElSwitch
            size="small"
            :model-value="row.enabled_flag"
            :loading="togglingKey === `${row.menu_ulid}:enabled_flag`"
            :disabled="Boolean(togglingKey)"
            @change="toggleFlag(asRow<MenuRow>(row), 'enabled_flag', Boolean($event))"
          />
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDrawer?.open(asRow<NWMenuVo>(row))">
            编辑
          </ElButton>
          <ElTooltip
            v-if="row.child_count > 0"
            :content="`该菜单下有 ${row.child_count} 个下级，请先删除下级`"
            placement="top"
          >
            <span>
              <ElButton size="small" text type="danger" disabled>删除</ElButton>
            </span>
          </ElTooltip>
          <ElButton
            v-else
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(asRow<MenuRow>(row).menu_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(asRow<MenuRow>(row))"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无菜单</span>
      </template>
    </ElTable>

    <MenuFormDrawer ref="formDrawer" :menu-list="menuList" @saved="handleSaved" />
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

    &__caret {
      padding: 0;
      width: 14px;
      height: 14px;
      flex: none;
    }

    &__caret-placeholder {
      width: 14px;
      flex: none;
    }

    &__code {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }
  }
</style>
