<!--
  技能参考资料面板：按技能维护参考资料（名称 / 触发关键词 / 正文 / 排序号）。
  关键词为空表示总是注入；面板直接读写后端，保存后回调父级刷新资料条数。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteSkillReference,
  listSkillReferences,
  saveSkillReference,
} from '@/api/skill'
import type { NWSkillReferenceVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const props = defineProps<{
  skill_ulid: string
  disabled_flag: boolean
}>()

const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()

const referenceList = ref<NWSkillReferenceVo[]>([])
const referenceLoading = ref(false)
const referenceDialogVisible = ref(false)
const referenceSaving = ref(false)
const editingRefUlid = ref('')

/** 参考资料表单（弹窗内编辑） */
const formState = ref({
  ref_name: '',
  ref_text: '',
  keyword_list: '',
  sort_order: 0,
})

/** 加载当前技能的参考资料 */
async function loadReferenceList(): Promise<void> {
  if (!props.skill_ulid) {
    referenceList.value = []
    return
  }
  referenceLoading.value = true
  try {
    referenceList.value = await listSkillReferences(props.skill_ulid)
  } catch {
    referenceList.value = []
  } finally {
    referenceLoading.value = false
  }
}

/** 打开弹窗：传资料为编辑，不传为新增 */
function openDialog(referenceItem?: NWSkillReferenceVo): void {
  editingRefUlid.value = referenceItem?.ref_ulid ?? ''
  formState.value = {
    ref_name: referenceItem?.ref_name ?? '',
    ref_text: referenceItem?.ref_text ?? '',
    keyword_list: referenceItem?.keyword_list ?? '',
    sort_order: referenceItem?.sort_order ?? 0,
  }
  referenceDialogVisible.value = true
}

async function handleSave(): Promise<void> {
  if (!formState.value.ref_name.trim() || !formState.value.ref_text.trim()) {
    ElMessage.warning(t('skills.ruleRefRequired'))
    return
  }
  referenceSaving.value = true
  try {
    await saveSkillReference({
      ref_ulid: editingRefUlid.value || undefined,
      skill_ulid: props.skill_ulid,
      ref_name: formState.value.ref_name.trim(),
      ref_text: formState.value.ref_text,
      keyword_list: formState.value.keyword_list.trim(),
      sort_order: formState.value.sort_order,
    })
    ElMessage.success(t('skills.refSaveSuccess'))
    referenceDialogVisible.value = false
    await loadReferenceList()
    emit('changed')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    referenceSaving.value = false
  }
}

async function handleDelete(referenceItem: NWSkillReferenceVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('skills.refDeleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteSkillReference(referenceItem.ref_ulid)
    ElMessage.success(t('skills.refDeleteSuccess'))
    await loadReferenceList()
    emit('changed')
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

watch(() => props.skill_ulid, loadReferenceList, { immediate: true })

defineExpose({ openDialog })
</script>

<template>
  <div class="skill-reference-panel">
    <div class="skill-reference-panel__head">
      <span class="skill-reference-panel__title">{{ t('skills.referenceTitle') }}</span>
      <NWButton size="small" :disabled="disabled_flag" @click="openDialog()">
        {{ t('skills.referenceAdd') }}
      </NWButton>
    </div>
    <p class="skill-reference-panel__hint">{{ t('skills.referenceHint') }}</p>

    <NWLoading v-if="referenceLoading" />
    <el-table v-else :data="referenceList" size="small">
      <el-table-column prop="ref_name" :label="t('skills.referenceName')" width="140" />
      <el-table-column :label="t('skills.referenceKeyword')" width="160">
        <template #default="{ row }">
          <span v-if="row.keyword_list">{{ row.keyword_list }}</span>
          <el-tag v-else size="small" type="info">{{ t('skills.referenceAlways') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('skills.referenceText')" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">{{ row.ref_text }}</template>
      </el-table-column>
      <el-table-column :label="t('common.action')" width="130">
        <template #default="{ row }">
          <NWButton size="small" :disabled="disabled_flag" @click="openDialog(row)">
            {{ t('common.edit') }}
          </NWButton>
          <NWButton type="danger-text" size="small" :disabled="disabled_flag" @click="handleDelete(row)">
            {{ t('common.delete') }}
          </NWButton>
        </template>
      </el-table-column>
      <template #empty>
        <NWEmpty :description_text="disabled_flag ? t('skills.saveFirst') : t('skills.referenceEmpty')" />
      </template>
    </el-table>

    <el-dialog
      v-model="referenceDialogVisible"
      :title="editingRefUlid ? t('skills.referenceEditTitle') : t('skills.referenceCreateTitle')"
      width="560px"
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item :label="t('skills.referenceName')">
          <el-input v-model="formState.ref_name" :maxlength="64" />
        </el-form-item>
        <el-form-item :label="t('skills.referenceKeyword')">
          <el-input v-model="formState.keyword_list" :placeholder="t('skills.referenceKeywordPlaceholder')" />
          <p class="skill-reference-panel__hint">{{ t('skills.referenceKeywordHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('skills.referenceText')">
          <el-input v-model="formState.ref_text" type="textarea" :rows="8" :maxlength="20000" show-word-limit />
        </el-form-item>
        <el-form-item :label="t('skills.sortOrder')">
          <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="referenceDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="referenceSaving" @click="handleSave">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped src="./skill-reference-panel.css"></style>
