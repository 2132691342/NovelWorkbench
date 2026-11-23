<!-- 模型授权弹窗：按角色与按用户两种粒度分配模型使用权限（用户直授优先于角色授权）。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { listAdminRoles, listAdminUsers, type NWRoleVo, type NWUserAdminVo } from '@/api/admin'
  import { grantModelCatalog, type NWModelCatalogVo } from '@/api/model'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const loading = ref(false)
  const saving = ref(false)
  const activeTab = ref<'role' | 'user'>('role')
  const roleList = ref<NWRoleVo[]>([])
  const userList = ref<NWUserAdminVo[]>([])
  const selectedRoleUlidList = ref<string[]>([])
  const selectedUserUlidList = ref<string[]>([])
  const model = ref<NWModelCatalogVo | null>(null)

  const title = computed(() =>
    model.value ? `授权使用 · ${model.value.model_label || model.value.model_name}` : '授权使用'
  )

  const loadOptions = async () => {
    loading.value = true
    try {
      const [rolePage, userPage] = await Promise.all([
        listAdminRoles({ page_num: 1, page_size: 200 }),
        listAdminUsers({ page_num: 1, page_size: 200 })
      ])
      roleList.value = rolePage.items ?? []
      userList.value = userPage.items ?? []
    } catch {
      roleList.value = []
      userList.value = []
    } finally {
      loading.value = false
    }
  }

  /** 打开授权弹窗：带上模型当前的授权状态 */
  const open = async (target: NWModelCatalogVo) => {
    model.value = target
    selectedRoleUlidList.value = [...(target.role_ulid_list ?? [])]
    selectedUserUlidList.value = [...(target.user_ulid_list ?? [])]
    activeTab.value = 'role'
    visible.value = true
    await loadOptions()
  }

  const submit = async () => {
    if (!model.value) return
    saving.value = true
    try {
      await grantModelCatalog(
        model.value.model_ulid,
        selectedRoleUlidList.value,
        selectedUserUlidList.value
      )
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
  <ElDialog v-model="visible" :title="title" width="620px" append-to-body destroy-on-close>
    <div v-loading="loading" class="grant-dialog__body">
      <ElAlert type="info" :closable="false" show-icon>
        管理员不受限制。角色授权随角色变更生效；用户直授独立于角色，用户被删除时其直授自动清理。
      </ElAlert>
      <ElTabs v-model="activeTab">
        <ElTabPane name="role" :label="`按角色（${selectedRoleUlidList.length}）`">
          <ElCheckboxGroup v-model="selectedRoleUlidList" class="grant-dialog__group">
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
          <ElEmpty v-if="!loading && roleList.length === 0" description="暂无可授权角色" :image-size="60" />
        </ElTabPane>
        <ElTabPane name="user" :label="`按用户（${selectedUserUlidList.length}）`">
          <ElCheckboxGroup v-model="selectedUserUlidList" class="grant-dialog__group">
            <ElCheckbox
              v-for="user in userList"
              :key="user.user_ulid"
              :value="user.user_ulid"
              class="grant-dialog__item"
            >
              <span class="grant-dialog__name">{{ user.user_name }}</span>
              <span class="grant-dialog__code">{{ user.user_email }}</span>
              <ElTag v-if="user.user_status_code !== 'active'" size="small" type="danger" effect="plain">
                已禁用
              </ElTag>
            </ElCheckbox>
          </ElCheckboxGroup>
          <ElEmpty v-if="!loading && userList.length === 0" description="暂无可授权用户" :image-size="60" />
        </ElTabPane>
      </ElTabs>
    </div>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">
        保存（角色 {{ selectedRoleUlidList.length }} · 用户 {{ selectedUserUlidList.length }}）
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
      min-height: 200px;
    }

    &__group {
      display: flex;
      flex-direction: column;
      gap: 4px;
      max-height: 320px;
      padding-top: 8px;
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
      font-size: 12px;
      color: var(--nw-text-muted);
    }
  }
</style>
