<!-- 角色授权弹窗：为模型分配可用角色。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import {
    ElButton,
    ElCheckbox,
    ElCheckboxGroup,
    ElDialog,
    ElEmpty,
    ElMessage,
    ElTag
  } from 'element-plus'
  import { listAdminRoles, type NWRoleVo } from '@/api/admin'
  import { grantModelCatalog, type NWModelCatalogVo } from '@/api/model'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const loading = ref(false)
  const saving = ref(false)
  const roleList = ref<NWRoleVo[]>([])
  const selectedRoleUlidList = ref<string[]>([])
  const model = ref<NWModelCatalogVo | null>(null)

  const title = computed(() =>
    model.value ? `授权角色 · ${model.value.model_label || model.value.model_name}` : '授权角色'
  )

  const loadRoles = async () => {
    loading.value = true
    try {
      const page = await listAdminRoles({ page_num: 1, page_size: 200 })
      roleList.value = page.items ?? []
    } catch {
      roleList.value = []
    } finally {
      loading.value = false
    }
  }

  /** 打开授权弹窗：带上模型当前已授权的角色 */
  const open = async (target: NWModelCatalogVo) => {
    model.value = target
    selectedRoleUlidList.value = [...(target.role_ulid_list ?? [])]
    visible.value = true
    await loadRoles()
  }

  const submit = async () => {
    if (!model.value) return
    saving.value = true
    try {
      await grantModelCatalog(model.value.model_ulid, selectedRoleUlidList.value)
      ElMessage.success('授权已更新')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="520px" append-to-body destroy-on-close>
    <div v-loading="loading" class="grant-dialog__body">
      <p class="nw-hint">勾选可使用该模型的角色；管理员不受限制，留空表示仅管理员可用。</p>
      <ElEmpty v-if="!loading && roleList.length === 0" description="暂无可授权角色" :image-size="64" />
      <ElCheckboxGroup v-else v-model="selectedRoleUlidList" class="grant-dialog__group">
        <ElCheckbox
          v-for="role in roleList"
          :key="role.role_ulid"
          :value="role.role_ulid"
          class="grant-dialog__item"
        >
          <span class="grant-dialog__name">{{ role.role_name }}</span>
          <ElTag v-if="role.builtin_flag" size="small" type="info" effect="plain">内置</ElTag>
          <span class="grant-dialog__code">{{ role.role_code }}</span>
        </ElCheckbox>
      </ElCheckboxGroup>
    </div>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">
        保存授权（{{ selectedRoleUlidList.length }}）
      </ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .grant-dialog {
    &__body {
      display: flex;
      flex-direction: column;
      gap: 10px;
      min-height: 120px;
    }

    &__group {
      display: flex;
      flex-direction: column;
      gap: 4px;
      max-height: 340px;
      overflow-y: auto;
    }

    &__item {
      height: 30px;
      margin-right: 0;
    }

    &__name {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__code {
      margin-left: 8px;
      font-size: 11px;
      color: var(--nw-text-muted);
    }
  }
</style>
