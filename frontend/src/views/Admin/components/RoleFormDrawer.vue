<!-- 角色权限抽屉：基本信息 + 按权限目录分组勾选（搜索、全选/清空、已选统计）。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElCheckbox,
    ElCheckboxGroup,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { saveAdminRole, type NWRoleVo } from '@/api/admin'
  import { fetchPermCatalog } from '@/api/menu'
  import type { NWPermGroupVo } from '@/api/contracts'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWRoleVo | null>(null)
  const permGroups = ref<NWPermGroupVo[]>([])
  const catalogLoading = ref(false)
  const permKeyword = ref('')

  const form = reactive({
    role_code: '',
    role_name: '',
    remark_text: '',
    perm_codes: [] as string[]
  })

  const title = computed(() => (editing.value ? '编辑角色' : '新建角色'))
  const builtinFlag = computed(() => editing.value?.builtin_flag === true)
  // 只有内置管理员锁定权限面板：其余内置角色（如普通用户）的权限必须可反复编辑
  const adminLocked = computed(() => editing.value?.role_code === 'admin')

  const rules = computed<FormRules>(() => ({
    role_code: [
      { required: true, message: '请输入角色编码', trigger: 'blur' },
      { max: 64, message: '最长 64 个字符', trigger: 'blur' }
    ],
    role_name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }]
  }))

  /** 权限目录（分组 → 权限项），只加载一次 */
  const loadCatalog = async () => {
    if (permGroups.value.length > 0) return
    catalogLoading.value = true
    try {
      permGroups.value = await fetchPermCatalog()
    } catch {
      permGroups.value = []
    } finally {
      catalogLoading.value = false
    }
  }

  /** 可勾选的分组：通配权限 "*" 只属于管理员，其他角色里不展示，避免选了必被后端拒绝 */
  const selectableGroups = computed(() =>
    permGroups.value
      .map((group) => ({
        ...group,
        perm_list: group.perm_list.filter((item) => item.perm_code !== '*')
      }))
      .filter((group) => group.perm_list.length > 0)
  )

  /** 搜索后的分组：按权限码/名称/说明过滤，空组不展示 */
  const visibleGroups = computed(() => {
    const keywordText = permKeyword.value.trim().toLowerCase()
    if (!keywordText) return selectableGroups.value
    return selectableGroups.value
      .map((group) => ({
        ...group,
        perm_list: group.perm_list.filter(
          (item) =>
            item.perm_code.toLowerCase().includes(keywordText) ||
            item.perm_name.toLowerCase().includes(keywordText) ||
            (item.perm_desc ?? '').toLowerCase().includes(keywordText)
        )
      }))
      .filter((group) => group.perm_list.length > 0)
  })

  const allPermCodes = computed(() =>
    selectableGroups.value.flatMap((group) => group.perm_list.map((item) => item.perm_code))
  )

  const selectedCount = computed(() => form.perm_codes.length)

  const groupCodes = (group: NWPermGroupVo) => group.perm_list.map((item) => item.perm_code)

  const isGroupChecked = (group: NWPermGroupVo) => {
    const codes = groupCodes(group)
    return codes.length > 0 && codes.every((code) => form.perm_codes.includes(code))
  }

  const isGroupIndeterminate = (group: NWPermGroupVo) => {
    const codes = groupCodes(group)
    const hitCount = codes.filter((code) => form.perm_codes.includes(code)).length
    return hitCount > 0 && hitCount < codes.length
  }

  const toggleGroup = (group: NWPermGroupVo, checked: boolean) => {
    const codes = groupCodes(group)
    const rest = form.perm_codes.filter((code) => !codes.includes(code))
    form.perm_codes = checked ? [...rest, ...codes] : rest
  }

  const selectAll = () => {
    form.perm_codes = [...allPermCodes.value]
  }

  const clearAll = () => {
    form.perm_codes = []
  }

  /** 打开抽屉：传角色为编辑，不传为新建；每次按当前行重建表单，避免残留上次勾选 */
  const open = async (role?: NWRoleVo) => {
    editing.value = role ?? null
    form.role_code = role?.role_code ?? ''
    form.role_name = role?.role_name ?? ''
    form.remark_text = role?.remark_text ?? ''
    form.perm_codes = (role?.perm_codes ?? []).filter(
      (code) => code !== '*' || role?.role_code === 'admin'
    )
    permKeyword.value = ''
    visible.value = true
    formRef.value?.clearValidate()
    await loadCatalog()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      await saveAdminRole({
        role_ulid: editing.value?.role_ulid,
        role_code: form.role_code,
        role_name: form.role_name,
        remark_text: form.remark_text,
        perm_codes: adminLocked.value ? ['*'] : [...form.perm_codes]
      })
      ElMessage.success(editing.value ? '角色已更新' : '角色已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="620px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="role-form__row">
        <ElFormItem label="角色编码" prop="role_code">
          <ElInput
            v-model="form.role_code"
            :disabled="builtinFlag"
            maxlength="64"
            placeholder="例如 editor"
          />
          <p v-if="builtinFlag" class="role-form__tip">内置角色的编码不可修改。</p>
        </ElFormItem>
        <ElFormItem label="角色名称" prop="role_name">
          <ElInput v-model="form.role_name" maxlength="64" placeholder="例如 内容编辑" />
        </ElFormItem>
      </div>
      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="2" maxlength="256" />
      </ElFormItem>

      <ElFormItem label="权限">
        <div v-loading="catalogLoading" class="role-form__perm">
          <ElAlert
            v-if="adminLocked"
            type="success"
            :closable="false"
            show-icon
            title="内置管理员拥有全部权限（*），无需逐项勾选"
          />
          <template v-else>
            <div class="role-form__perm-toolbar">
              <ElInput
                v-model="permKeyword"
                placeholder="搜索权限码或名称"
                clearable
                size="small"
                class="role-form__perm-search"
              />
              <span class="nw-hint">已选 {{ selectedCount }} 项</span>
              <div class="role-form__perm-spacer" />
              <ElButton size="small" text type="primary" @click="selectAll()">全选</ElButton>
              <ElButton size="small" text :disabled="selectedCount === 0" @click="clearAll()">
                清空
              </ElButton>
            </div>

            <div v-for="group in visibleGroups" :key="group.group_code" class="role-form__group">
              <label class="role-form__group-head">
                <ElCheckbox
                  :model-value="isGroupChecked(group)"
                  :indeterminate="isGroupIndeterminate(group)"
                  @change="toggleGroup(group, Boolean($event))"
                />
                <span class="role-form__group-name">{{ group.group_name }}</span>
                <span class="nw-hint">{{ group.perm_list.length }} 项</span>
              </label>
              <ElCheckboxGroup v-model="form.perm_codes" class="role-form__items">
                <ElCheckbox
                  v-for="item in group.perm_list"
                  :key="item.perm_code"
                  :value="item.perm_code"
                  :title="item.perm_desc"
                >
                  {{ item.perm_name }}
                </ElCheckbox>
              </ElCheckboxGroup>
            </div>
            <p v-if="!catalogLoading && visibleGroups.length === 0" class="nw-hint">
              {{ permKeyword ? '没有匹配的权限项' : '暂无可用权限目录' }}
            </p>
          </template>
        </div>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .role-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__tip {
      margin-top: 4px;
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__perm {
      width: 100%;
      max-height: 420px;
      padding-right: 4px;
      overflow-y: auto;
    }

    &__perm-toolbar {
      display: flex;
      gap: 8px;
      align-items: center;
      position: sticky;
      top: 0;
      z-index: 1;
      padding: 6px 0 10px;
      background: var(--nw-surface);
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__perm-search {
      width: 200px;
    }

    &__perm-spacer {
      flex: 1;
    }

    &__group {
      padding: 10px 0;
      border-bottom: 1px solid var(--nw-border-soft);

      &:last-child {
        border-bottom: 0;
      }
    }

    &__group-head {
      display: flex;
      gap: 8px;
      align-items: center;
      margin-bottom: 6px;
      cursor: pointer;
    }

    &__group-name {
      font-size: 13px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__items {
      display: flex;
      flex-wrap: wrap;
      gap: 4px 16px;
      padding-left: 24px;
    }
  }
</style>
