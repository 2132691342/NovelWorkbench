<!-- 菜单管理页：菜单树维护（分组/页面/动作），菜单表同时是权限码登记处。 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { deleteMenu, listMenus, saveMenu, type NWMenuSaveReqVo } from '@/api/menu'
import type { NWMenuVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import { NW_ICON_CODE_LIST } from './menuIcons'

const { t } = useI18n()

/** 菜单类型与作用域选项 */
const NW_MENU_TYPE_LIST = ['group', 'page', 'action'] as const
const NW_SCOPE_LIST = ['global', 'project'] as const

/** 树节点：在菜单视图对象上附加层级深度与子节点 */
interface NWMenuTreeNode extends NWMenuVo {
  depth_num: number
  child_list: NWMenuTreeNode[]
}

const menuList = ref<NWMenuVo[]>([])
const listLoading = ref(false)
const keywordText = ref('')
const typeFilter = ref('')

const drawerVisible = ref(false)
const saving = ref(false)
const deletingKey = ref('')
const formRef = ref<FormInstance>()

const formState = reactive({
  menu_ulid: '',
  parent_ulid: '',
  menu_code: '',
  menu_name: '',
  menu_type_code: 'page',
  scope_code: 'global',
  route_path: '',
  icon_code: '',
  perm_code: '',
  visible_flag: true,
  enabled_flag: true,
  sort_order: 0,
  remark_text: '',
})

const formRules = computed<FormRules>(() => ({
  menu_code: [{ required: true, message: t('admin.menus.ruleCodeRequired'), trigger: 'blur' }],
  menu_name: [{ required: true, message: t('admin.menus.ruleNameRequired'), trigger: 'blur' }],
}))

/** 把平铺菜单组装成树（父级缺失的节点提升为根，避免数据异常时整棵树消失） */
const menuTree = computed<NWMenuTreeNode[]>(() => {
  const nodeMap = new Map<string, NWMenuTreeNode>()
  for (const menuItem of menuList.value) {
    nodeMap.set(menuItem.menu_ulid, { ...menuItem, depth_num: 0, child_list: [] })
  }
  const rootList: NWMenuTreeNode[] = []
  for (const menuItem of menuList.value) {
    const node = nodeMap.get(menuItem.menu_ulid)
    if (!node) {
      continue
    }
    const parentNode = menuItem.parent_ulid ? nodeMap.get(menuItem.parent_ulid) : undefined
    if (parentNode) {
      parentNode.child_list.push(node)
    } else {
      rootList.push(node)
    }
  }
  const assignDepth = (nodeList: NWMenuTreeNode[], depth: number): void => {
    for (const node of nodeList) {
      node.depth_num = depth
      node.child_list.sort((left, right) => left.sort_order - right.sort_order)
      assignDepth(node.child_list, depth + 1)
    }
  }
  rootList.sort((left, right) => left.sort_order - right.sort_order)
  assignDepth(rootList, 0)
  return rootList
})

/** 上级菜单候选（仅分组与页面可作为父级） */
const parentOptionList = computed(() =>
  menuList.value.filter(
    (menuItem) => menuItem.menu_type_code !== 'action' && menuItem.menu_ulid !== formState.menu_ulid,
  ),
)

function typeLabel(menuTypeCode: string): string {
  switch (menuTypeCode) {
    case 'group':
      return t('admin.menus.typeGroup')
    case 'page':
      return t('admin.menus.typePage')
    default:
      return t('admin.menus.typeAction')
  }
}

function typeTagType(menuTypeCode: string): 'primary' | 'success' | 'warning' {
  if (menuTypeCode === 'group') {
    return 'warning'
  }
  return menuTypeCode === 'page' ? 'primary' : 'success'
}

async function loadMenuList(): Promise<void> {
  listLoading.value = true
  try {
    menuList.value = await listMenus({
      keyword_text: keywordText.value.trim() || undefined,
      menu_type_code: typeFilter.value || undefined,
    })
  } catch {
    // 失败提示由 http 层统一弹出，保留当前列表
  } finally {
    listLoading.value = false
  }
}

function resetFormState(): void {
  formState.menu_ulid = ''
  formState.parent_ulid = ''
  formState.menu_code = ''
  formState.menu_name = ''
  formState.menu_type_code = 'page'
  formState.scope_code = 'global'
  formState.route_path = ''
  formState.icon_code = ''
  formState.perm_code = ''
  formState.visible_flag = true
  formState.enabled_flag = true
  formState.sort_order = 0
  formState.remark_text = ''
}

/** 新建：带父级时预填上级与作用域，减少重复输入 */
function handleCreateMenu(parentMenu?: NWMenuVo): void {
  resetFormState()
  if (parentMenu) {
    formState.parent_ulid = parentMenu.menu_ulid
    formState.scope_code = parentMenu.scope_code
  }
  drawerVisible.value = true
}

function handleEditMenu(menuItem: NWMenuVo): void {
  resetFormState()
  formState.menu_ulid = menuItem.menu_ulid
  formState.parent_ulid = menuItem.parent_ulid
  formState.menu_code = menuItem.menu_code
  formState.menu_name = menuItem.menu_name
  formState.menu_type_code = menuItem.menu_type_code
  formState.scope_code = menuItem.scope_code
  formState.route_path = menuItem.route_path
  formState.icon_code = menuItem.icon_code
  formState.perm_code = menuItem.perm_code
  formState.visible_flag = menuItem.visible_flag
  formState.enabled_flag = menuItem.enabled_flag
  formState.sort_order = menuItem.sort_order
  formState.remark_text = menuItem.remark_text
  drawerVisible.value = true
}

async function handleSaveMenu(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    const saveReqVo: NWMenuSaveReqVo = {
      menu_code: formState.menu_code.trim(),
      menu_name: formState.menu_name.trim(),
      menu_type_code: formState.menu_type_code,
      scope_code: formState.scope_code,
      route_path: formState.route_path.trim(),
      icon_code: formState.icon_code,
      perm_code: formState.perm_code.trim(),
      visible_flag: formState.visible_flag,
      enabled_flag: formState.enabled_flag,
      sort_order: formState.sort_order,
      remark_text: formState.remark_text,
    }
    if (formState.parent_ulid) {
      saveReqVo.parent_ulid = formState.parent_ulid
    }
    if (formState.menu_ulid) {
      saveReqVo.menu_ulid = formState.menu_ulid
    }
    await saveMenu(saveReqVo)
    ElMessage.success(t('admin.menus.saveSuccess'))
    drawerVisible.value = false
    await loadMenuList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    saving.value = false
  }
}

async function handleDeleteMenu(menuItem: NWMenuVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('admin.menus.deleteConfirm', { name: menuItem.menu_name }),
      t('common.confirm'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' },
    )
  } catch {
    return
  }
  deletingKey.value = menuItem.menu_ulid
  try {
    await deleteMenu(menuItem.menu_ulid)
    ElMessage.success(t('admin.menus.deleteSuccess'))
    await loadMenuList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    deletingKey.value = ''
  }
}

function handleSearch(): void {
  void loadMenuList()
}

function handleResetFilters(): void {
  keywordText.value = ''
  typeFilter.value = ''
  void loadMenuList()
}

onMounted(() => {
  void loadMenuList()
})
</script>

<template>
  <div class="nw-page admin-menus-page">
    <NWPageHeader
      :title_text="t('admin.menus.title')"
      :subtitle_text="t('admin.menus.pageSubtitle')"
      icon_code="menu"
    >
      <template #actions>
        <NWButton type="primary" @click="handleCreateMenu()">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('common.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <NWCard :hoverable="false">
      <template #header>
        <div class="admin-menus-page__header-actions">
          <el-input
            v-model="keywordText"
            class="admin-menus-page__search"
            :placeholder="t('admin.menus.keywordPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          />
          <el-select v-model="typeFilter" class="admin-menus-page__filter" :placeholder="t('admin.menus.typeAll')" clearable>
            <el-option
              v-for="menuType in NW_MENU_TYPE_LIST"
              :key="menuType"
              :value="menuType"
              :label="typeLabel(menuType)"
            />
          </el-select>
          <NWButton @click="handleSearch">{{ t('admin.search') }}</NWButton>
          <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
        </div>
      </template>

      <p class="admin-menus-page__hint">{{ t('admin.menus.hint') }}</p>

      <NWLoading v-if="listLoading" />
      <el-tree
        v-else
        class="admin-menus-page__tree"
        :data="menuTree"
        node-key="menu_ulid"
        :props="{ label: 'menu_name', children: 'child_list' }"
        default-expand-all
        :expand-on-click-node="false"
      >
        <template #default="{ data }">
          <div class="admin-menus-page__node">
            <span class="admin-menus-page__node-name">{{ data.menu_name }}</span>
            <el-tag size="small" :type="typeTagType(data.menu_type_code)">
              {{ typeLabel(data.menu_type_code) }}
            </el-tag>
            <el-tag size="small" effect="plain">{{ data.menu_code }}</el-tag>
            <el-tag v-if="data.perm_code" size="small" type="info" effect="plain">
              {{ data.perm_code }}
            </el-tag>
            <span v-if="data.route_path" class="admin-menus-page__node-path">{{ data.route_path }}</span>
            <el-tag v-if="!data.enabled_flag" size="small" type="danger" effect="plain">
              {{ t('admin.menus.disabledTag') }}
            </el-tag>
            <el-tag v-if="!data.visible_flag" size="small" type="warning" effect="plain">
              {{ t('admin.menus.hiddenTag') }}
            </el-tag>
            <span class="admin-menus-page__node-actions">
              <NWButton v-if="data.menu_type_code !== 'action'" size="small" @click.stop="handleCreateMenu(data)">
                {{ t('admin.menus.addChild') }}
              </NWButton>
              <NWButton size="small" @click.stop="handleEditMenu(data)">{{ t('common.edit') }}</NWButton>
              <NWButton
                type="danger-text"
                size="small"
                :loading="deletingKey === data.menu_ulid"
                @click.stop="handleDeleteMenu(data)"
              >
                {{ t('common.delete') }}
              </NWButton>
            </span>
          </div>
        </template>
      </el-tree>
      <NWEmpty v-if="!listLoading && !menuTree.length" :description_text="t('admin.menus.emptyTip')" />
    </NWCard>

    <!-- 编辑抽屉：菜单全字段 -->
    <el-drawer
      v-model="drawerVisible"
      :title="formState.menu_ulid ? t('admin.menus.editTitle') : t('admin.menus.createTitle')"
      size="520px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <div class="admin-menus-page__grid">
          <el-form-item prop="menu_name" :label="t('admin.menus.menuName')">
            <el-input v-model="formState.menu_name" :maxlength="64" show-word-limit />
          </el-form-item>
          <el-form-item prop="menu_code" :label="t('admin.menus.menuCode')">
            <el-input v-model="formState.menu_code" :maxlength="64" :disabled="Boolean(formState.menu_ulid)" />
          </el-form-item>
        </div>

        <div class="admin-menus-page__grid">
          <el-form-item :label="t('admin.menus.menuType')">
            <el-select v-model="formState.menu_type_code" class="admin-menus-page__full">
              <el-option v-for="menuType in NW_MENU_TYPE_LIST" :key="menuType" :value="menuType" :label="typeLabel(menuType)" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('admin.menus.scope')">
            <el-select v-model="formState.scope_code" class="admin-menus-page__full">
              <el-option v-for="scopeCode in NW_SCOPE_LIST" :key="scopeCode" :value="scopeCode" :label="scopeCode" />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item :label="t('admin.menus.parentMenu')">
          <el-select v-model="formState.parent_ulid" class="admin-menus-page__full" clearable :placeholder="t('admin.menus.parentNone')">
            <el-option
              v-for="parentItem in parentOptionList"
              :key="parentItem.menu_ulid"
              :value="parentItem.menu_ulid"
              :label="`${parentItem.menu_name} (${parentItem.menu_code})`"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="t('admin.menus.routePath')">
          <el-input v-model="formState.route_path" placeholder="/workspace/:project_ulid" />
          <p class="admin-menus-page__field-hint">{{ t('admin.menus.routePathHint') }}</p>
        </el-form-item>

        <div class="admin-menus-page__grid">
          <el-form-item :label="t('admin.menus.iconCode')">
            <el-select v-model="formState.icon_code" class="admin-menus-page__full" clearable filterable>
              <el-option v-for="iconCode in NW_ICON_CODE_LIST" :key="iconCode" :value="iconCode" :label="iconCode" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('admin.menus.sortOrder')">
            <el-input-number v-model="formState.sort_order" :min="0" :max="9999" class="admin-menus-page__full" />
          </el-form-item>
        </div>

        <el-form-item :label="t('admin.menus.permCode')">
          <el-input v-model="formState.perm_code" placeholder="system:user:manage" />
          <p class="admin-menus-page__field-hint">{{ t('admin.menus.permCodeHint') }}</p>
        </el-form-item>

        <div class="admin-menus-page__grid">
          <el-form-item :label="t('admin.menus.visibleFlag')">
            <el-switch v-model="formState.visible_flag" />
          </el-form-item>
          <el-form-item :label="t('admin.menus.enabledFlag')">
            <el-switch v-model="formState.enabled_flag" />
          </el-form-item>
        </div>

        <el-form-item :label="t('admin.menus.remark')">
          <el-input v-model="formState.remark_text" type="textarea" :rows="2" :maxlength="256" show-word-limit />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="admin-menus-page__drawer-footer">
          <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
          <NWButton type="primary" :loading="saving" @click="handleSaveMenu">{{ t('common.save') }}</NWButton>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.admin-menus-page__header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.admin-menus-page__search {
  width: 200px;
}

.admin-menus-page__filter {
  width: 130px;
}

.admin-menus-page__hint {
  margin: 0 0 14px;
  color: var(--text-secondary);
  font-size: 12.5px;
  line-height: 1.7;
}

.admin-menus-page__tree {
  background-color: transparent;
}

.admin-menus-page__node {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 3px 0;
}

.admin-menus-page__node-name {
  min-width: 96px;
  font-size: 13.5px;
  font-weight: 500;
}

.admin-menus-page__node-path {
  color: var(--text-placeholder);
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.admin-menus-page__node-actions {
  display: flex;
  gap: 6px;
  margin-left: auto;
  padding-right: 8px;
}

.admin-menus-page__grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.admin-menus-page__full {
  width: 100%;
}

.admin-menus-page__field-hint {
  margin: 4px 0 0;
  color: var(--text-placeholder);
  font-size: 12px;
  line-height: 1.6;
}

.admin-menus-page__drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
