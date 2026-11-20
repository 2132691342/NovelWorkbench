<!--
  模型角色分配对话框：按角色勾选该模型的使用权限，保存即整体覆盖（取消全选=收回权限）。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWRoleVo } from '@/api/admin'
import type { NWModelCatalogVo } from '@/api/model'
import NWButton from '@/components/NWButton.vue'

const emit = defineEmits<{ saved: [modelUlid: string, roleUlidList: string[]] }>()

const { t } = useI18n()

const dialogVisible = ref(false)
const savingFlag = ref(false)
const modelVo = ref<NWModelCatalogVo | null>(null)
const roleVoList = ref<NWRoleVo[]>([])
const selectedRoleUlidList = ref<string[]>([])

/** 已选角色数量（对话框底部摘要） */
const selectedCount = computed(() => selectedRoleUlidList.value.length)

/** 打开对话框（预填当前授权） */
function open(targetVo: NWModelCatalogVo, roles: NWRoleVo[]): void {
  modelVo.value = targetVo
  roleVoList.value = roles
  selectedRoleUlidList.value = [...targetVo.role_ulid_list]
  dialogVisible.value = true
}

/** 全选/清空 */
function toggleAll(checkedFlag: boolean): void {
  selectedRoleUlidList.value = checkedFlag ? roleVoList.value.map((item) => item.role_ulid) : []
}

/** 保存授权 */
function handleSubmit(): void {
  if (!modelVo.value) {
    return
  }
  savingFlag.value = true
  emit('saved', modelVo.value.model_ulid, selectedRoleUlidList.value)
  savingFlag.value = false
  dialogVisible.value = false
}

/** 父组件保存成功后关闭加载态 */
function closeDialog(): void {
  savingFlag.value = false
}

defineExpose({ open, closeDialog })
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="t('models.grantTitle')"
    width="560px"
    align-center
  >
    <div v-if="modelVo" class="grant-dialog">
      <p class="grant-dialog__model">
        {{ modelVo.model_label || modelVo.model_name }}
        <span class="grant-dialog__provider">{{ modelVo.provider_name }}</span>
      </p>
      <div class="grant-dialog__toolbar">
        <span class="grant-dialog__hint">{{ t('models.grantHint') }}</span>
        <div class="grant-dialog__tools">
          <NWButton size="small" type="text" @click="toggleAll(true)">{{ t('models.grantAll') }}</NWButton>
          <NWButton size="small" type="text" @click="toggleAll(false)">{{ t('models.grantClear') }}</NWButton>
        </div>
      </div>
      <el-checkbox-group v-model="selectedRoleUlidList" class="grant-dialog__list">
        <label
          v-for="roleVo in roleVoList"
          :key="roleVo.role_ulid"
          class="grant-dialog__item"
          :class="{ 'is-checked': selectedRoleUlidList.includes(roleVo.role_ulid) }"
        >
          <el-checkbox :value="roleVo.role_ulid">
            <span class="grant-dialog__role-name">{{ roleVo.role_name }}</span>
            <span class="grant-dialog__role-code">{{ roleVo.role_code }}</span>
          </el-checkbox>
        </label>
      </el-checkbox-group>
      <p v-if="roleVoList.length === 0" class="grant-dialog__empty">{{ t('models.grantNoRole') }}</p>
    </div>
    <template #footer>
      <span class="grant-dialog__count">{{ t('models.grantSelected', { count: selectedCount }) }}</span>
      <NWButton @click="dialogVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="savingFlag" @click="handleSubmit">{{ t('common.save') }}</NWButton>
    </template>
  </el-dialog>
</template>

<style scoped src="./modelGrant.css"></style>
