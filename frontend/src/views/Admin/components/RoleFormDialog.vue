<!-- 角色编辑弹窗：维护角色信息与权限勾选。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElCheckbox,
    ElCheckboxGroup,
    ElDialog,
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

  const form = reactive({
    role_code: '',
    role_name: '',
    remark_text: '',
    perm_codes: [] as string[]
  })

  const title = computed(() => (editing.value ? '编辑角色' : '新建角色'))
  const builtinFlag = computed(() => editing.value?.builtin_flag === true)
  const allPermFlag = computed(
    () => editing.value?.builtin_flag === true && (editing.value.perm_codes ?? []).includes('*')
  )

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

  const groupCodes = (group: NWPermGroupVo) => group.perm_list.map((item) => item.perm_code)

  const isGroupChecked = (group: NWPermGroupVo) =>
    groupCodes(group).every((code) => form.perm_codes.includes(code))

  const toggleGroup = (group: NWPermGroupVo, checked: boolean) => {
    const codes = groupCodes(group)
    const rest = form.perm_codes.filter((code) => !codes.includes(code))
    form.perm_codes = checked ? [...rest, ...codes] : rest
  }

  /** 打开弹窗：传角色为编辑，不传为新建 */
  const open = async (role?: NWRoleVo) => {
    editing.value = role ?? null
    form.role_code = role?.role_code ?? ''
    form.role_name = role?.role_name ?? ''
    form.remark_text = role?.remark_text ?? ''
    form.perm_codes = [...(role?.perm_codes ?? [])]
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
        perm_codes: allPermFlag.value ? ['*'] : form.perm_codes
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
  <ElDialog v-model="visible" :title="title" width="620px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="role-form__row">
        <ElFormItem label="角色编码" prop="role_code">
          <ElInput
            v-model="form.role_code"
            :disabled="builtinFlag"
            maxlength="64"
            placeholder="例如 editor"
          />
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
            v-if="allPermFlag"
            type="success"
            :closable="false"
            show-icon
            title="内置管理员拥有全部权限（*），无需逐项勾选"
          />
          <template v-else>
            <div v-for="group in permGroups" :key="group.group_code" class="role-form__group">
              <label class="role-form__group-head">
                <ElCheckbox
                  :model-value="isGroupChecked(group)"
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
            <p v-if="!catalogLoading && permGroups.length === 0" class="nw-hint">
              暂无可用权限目录
            </p>
          </template>
        </div>
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .role-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__perm {
      width: 100%;
      max-height: 320px;
      padding-right: 4px;
      overflow-y: auto;
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
