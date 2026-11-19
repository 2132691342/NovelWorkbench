<!-- 连续性看板三个编辑弹窗：时间事实、伏笔钩子、章节摘要。 -->
// 连续性看板对话框：时间事实、伏笔钩子与章节摘要的新建/编辑表单。

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { useI18n } from 'vue-i18n'
import NWButton from '@/components/NWButton.vue'
import { saveFact, saveHook, saveSummary } from '@/api/continuity'
import type { NWChapterBriefVo } from '@/api/contracts'
import type {
  NWFactStatusCode,
  NWHookStatusCode,
  NWImportanceCode,
  NWSummaryStatusCode,
  NWTemporalFactVo,
  NWForeshadowHookVo,
  NWChapterSummaryVo,
} from '@/api/continuity'

const { t } = useI18n()

const props = defineProps<{
  projectUlid: string
  chapterList: NWChapterBriefVo[]
  factStatusText: Record<NWFactStatusCode, string>
  hookStatusText: Record<NWHookStatusCode, string>
  summaryStatusText: Record<NWSummaryStatusCode, string>
  importanceText: Record<NWImportanceCode, string>
}>()

const emit = defineEmits<{
  factSaved: []
  hookSaved: []
  summarySaved: []
}>()

const factDialogVisible = ref(false)
const hookDialogVisible = ref(false)
const summaryDialogVisible = ref(false)
const factSaving = ref(false)
const hookSaving = ref(false)
const summarySaving = ref(false)
const editingFactUlid = ref('')
const editingHookUlid = ref('')
const editingSummaryUlid = ref('')
const factFormRef = ref<FormInstance>()
const hookFormRef = ref<FormInstance>()
const summaryFormRef = ref<FormInstance>()

const factForm = reactive({
  subject_text: '',
  predicate_code: '',
  object_text: '',
  fact_status_code: 'candidate' as NWFactStatusCode,
  valid_from_chapter_ulid: '',
  valid_until_chapter_ulid: '',
  source_chapter_ulid: '',
  source_quote_text: '',
  note_text: '',
})
const hookForm = reactive({
  hook_status_code: 'planned' as NWHookStatusCode,
  description_text: '',
  expected_payoff_text: '',
  setup_chapter_ulid: '',
  payoff_chapter_ulid: '',
  importance_code: 'medium' as NWImportanceCode,
  note_text: '',
})
const summaryForm = reactive({
  chapter_ulid: '',
  summary_text: '',
  source_hash_text: '',
  summary_status_code: 'pending' as NWSummaryStatusCode,
})

/** 打开时间事实表单 */
function openFact(item?: NWTemporalFactVo): void {
  editingFactUlid.value = item?.fact_ulid ?? ''
  Object.assign(factForm, {
    subject_text: item?.subject_text ?? '',
    predicate_code: item?.predicate_code ?? '',
    object_text: item?.object_text ?? '',
    fact_status_code: item?.fact_status_code ?? 'candidate',
    valid_from_chapter_ulid: item?.valid_from_chapter_ulid ?? '',
    valid_until_chapter_ulid: item?.valid_until_chapter_ulid ?? '',
    source_chapter_ulid: item?.source_chapter_ulid ?? '',
    source_quote_text: item?.source_quote_text ?? '',
    note_text: item?.note_text ?? '',
  })
  factDialogVisible.value = true
}

/** 打开伏笔钩子表单 */
function openHook(item?: NWForeshadowHookVo): void {
  editingHookUlid.value = item?.hook_ulid ?? ''
  Object.assign(hookForm, {
    hook_status_code: item?.hook_status_code ?? 'planned',
    description_text: item?.description_text ?? '',
    expected_payoff_text: item?.expected_payoff_text ?? '',
    setup_chapter_ulid: item?.setup_chapter_ulid ?? '',
    payoff_chapter_ulid: item?.payoff_chapter_ulid ?? '',
    importance_code: item?.importance_code ?? 'medium',
    note_text: item?.note_text ?? '',
  })
  hookDialogVisible.value = true
}

/** 打开章节摘要表单 */
function openSummary(item?: NWChapterSummaryVo): void {
  editingSummaryUlid.value = item?.summary_ulid ?? ''
  Object.assign(summaryForm, {
    chapter_ulid: item?.chapter_ulid ?? '',
    summary_text: item?.summary_text ?? '',
    source_hash_text: item?.source_hash_text ?? '',
    summary_status_code: item?.summary_status_code ?? 'pending',
  })
  summaryDialogVisible.value = true
}

/** 校验表单，通过返回 true */
async function validate(formRef: FormInstance | undefined): Promise<boolean> {
  if (!formRef) {
    return false
  }
  try {
    await formRef.validate()
    return true
  } catch {
    return false
  }
}

/** 保存时间事实 */
async function submitFact(): Promise<void> {
  if (!(await validate(factFormRef.value))) {
    return
  }
  factSaving.value = true
  try {
    await saveFact({
      fact_ulid: editingFactUlid.value || undefined,
      project_ulid: props.projectUlid,
      subject_text: factForm.subject_text.trim(),
      predicate_code: factForm.predicate_code.trim(),
      object_text: factForm.object_text.trim(),
      fact_status_code: factForm.fact_status_code,
      valid_from_chapter_ulid: factForm.valid_from_chapter_ulid.trim(),
      valid_until_chapter_ulid: factForm.valid_until_chapter_ulid.trim(),
      source_chapter_ulid: factForm.source_chapter_ulid.trim(),
      source_quote_text: factForm.source_quote_text,
      note_text: factForm.note_text,
    })
    ElMessage.success(t('continuity.saveSuccess'))
    factDialogVisible.value = false
    emit('factSaved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    factSaving.value = false
  }
}

/** 保存伏笔钩子（状态随显式保存推进） */
async function submitHook(): Promise<void> {
  if (!(await validate(hookFormRef.value))) {
    return
  }
  hookSaving.value = true
  try {
    await saveHook({
      hook_ulid: editingHookUlid.value || undefined,
      project_ulid: props.projectUlid,
      hook_status_code: hookForm.hook_status_code,
      description_text: hookForm.description_text.trim(),
      expected_payoff_text: hookForm.expected_payoff_text,
      setup_chapter_ulid: hookForm.setup_chapter_ulid.trim(),
      payoff_chapter_ulid: hookForm.payoff_chapter_ulid.trim(),
      importance_code: hookForm.importance_code,
      note_text: hookForm.note_text,
    })
    ElMessage.success(t('continuity.saveSuccess'))
    hookDialogVisible.value = false
    emit('hookSaved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    hookSaving.value = false
  }
}

/** 保存章节摘要 */
async function submitSummary(): Promise<void> {
  if (!(await validate(summaryFormRef.value))) {
    return
  }
  summarySaving.value = true
  try {
    await saveSummary({
      summary_ulid: editingSummaryUlid.value || undefined,
      project_ulid: props.projectUlid,
      chapter_ulid: summaryForm.chapter_ulid.trim(),
      summary_text: summaryForm.summary_text,
      source_hash_text: summaryForm.source_hash_text.trim(),
      summary_status_code: summaryForm.summary_status_code,
    })
    ElMessage.success(t('continuity.saveSuccess'))
    summaryDialogVisible.value = false
    emit('summarySaved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    summarySaving.value = false
  }
}

defineExpose({ openFact, openHook, openSummary })
</script>

<template>
  <!-- 时间事实表单 -->
  <el-dialog v-model="factDialogVisible" :title="editingFactUlid ? t('continuity.factEditTitle') : t('continuity.factCreateTitle')" width="560px">
    <el-form ref="factFormRef" :model="factForm" label-width="100px">
      <el-form-item :label="t('continuity.subject')" prop="subject_text" required>
        <el-input v-model="factForm.subject_text" maxlength="128" />
      </el-form-item>
      <el-form-item :label="t('continuity.predicate')" prop="predicate_code" required>
        <el-input v-model="factForm.predicate_code" maxlength="64" />
      </el-form-item>
      <el-form-item :label="t('continuity.object')">
        <el-input v-model="factForm.object_text" maxlength="256" />
      </el-form-item>
      <el-form-item :label="t('continuity.colStatus')">
        <el-select v-model="factForm.fact_status_code" class="continuity-form-control">
          <el-option v-for="(text, code) in props.factStatusText" :key="code" :label="text" :value="code" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.validFrom')">
        <el-select v-model="factForm.valid_from_chapter_ulid" class="continuity-form-control" clearable :placeholder="t('continuity.unlimited')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.validUntil')">
        <el-select v-model="factForm.valid_until_chapter_ulid" class="continuity-form-control" clearable :placeholder="t('continuity.unlimited')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.sourceChapter')" prop="source_chapter_ulid" required>
        <el-select v-model="factForm.source_chapter_ulid" class="continuity-form-control" filterable :placeholder="t('continuity.selectSourceChapter')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.quote')">
        <el-input v-model="factForm.source_quote_text" type="textarea" :rows="3" maxlength="8000" />
      </el-form-item>
      <el-form-item :label="t('continuity.remark')">
        <el-input v-model="factForm.note_text" type="textarea" :rows="2" maxlength="4000" />
      </el-form-item>
    </el-form>
    <template #footer>
      <NWButton @click="factDialogVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="factSaving" @click="submitFact">{{ t('common.save') }}</NWButton>
    </template>
  </el-dialog>

  <!-- 伏笔钩子表单 -->
  <el-dialog v-model="hookDialogVisible" :title="editingHookUlid ? t('continuity.hookEditTitle') : t('continuity.hookCreateTitle')" width="560px">
    <el-form ref="hookFormRef" :model="hookForm" label-width="100px">
      <el-form-item :label="t('continuity.hookDesc')" prop="description_text" required>
        <el-input v-model="hookForm.description_text" type="textarea" :rows="3" maxlength="4000" />
      </el-form-item>
      <el-form-item :label="t('continuity.colStatus')">
        <el-select v-model="hookForm.hook_status_code" class="continuity-form-control">
          <el-option v-for="(text, code) in props.hookStatusText" :key="code" :label="text" :value="code" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.importance')">
        <el-select v-model="hookForm.importance_code" class="continuity-form-control">
          <el-option v-for="(text, code) in props.importanceText" :key="code" :label="text" :value="code" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.setupChapter')">
        <el-select v-model="hookForm.setup_chapter_ulid" class="continuity-form-control" clearable filterable :placeholder="t('continuity.selectSetupChapter')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.payoffChapter')">
        <el-select v-model="hookForm.payoff_chapter_ulid" class="continuity-form-control" clearable filterable :placeholder="t('continuity.selectPayoffChapter')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.expectedPayoff')">
        <el-input v-model="hookForm.expected_payoff_text" type="textarea" :rows="3" maxlength="4000" />
      </el-form-item>
      <el-form-item :label="t('continuity.remark')">
        <el-input v-model="hookForm.note_text" type="textarea" :rows="2" maxlength="4000" />
      </el-form-item>
    </el-form>
    <template #footer>
      <NWButton @click="hookDialogVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="hookSaving" @click="submitHook">{{ t('common.save') }}</NWButton>
    </template>
  </el-dialog>

  <!-- 章节摘要表单 -->
  <el-dialog v-model="summaryDialogVisible" :title="editingSummaryUlid ? t('continuity.summaryEditTitle') : t('continuity.summaryCreateTitle')" width="560px">
    <el-form ref="summaryFormRef" :model="summaryForm" label-width="100px">
      <el-form-item :label="t('continuity.summaryChapter')" prop="chapter_ulid" required>
        <el-select v-model="summaryForm.chapter_ulid" class="continuity-form-control" filterable :placeholder="t('continuity.selectChapter')">
          <el-option v-for="chapterItem in props.chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('continuity.summaryContent')" prop="summary_text" required>
        <el-input v-model="summaryForm.summary_text" type="textarea" :rows="8" maxlength="20000" />
      </el-form-item>
      <el-form-item :label="t('continuity.sourceHash')">
        <el-input v-model="summaryForm.source_hash_text" maxlength="128" />
      </el-form-item>
      <el-form-item :label="t('continuity.colStatus')">
        <el-select v-model="summaryForm.summary_status_code" class="continuity-form-control">
          <el-option v-for="(text, code) in props.summaryStatusText" :key="code" :label="text" :value="code" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <NWButton @click="summaryDialogVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="summarySaving" @click="submitSummary">{{ t('common.save') }}</NWButton>
    </template>
  </el-dialog>
</template>

<style scoped src="./continuity.css"></style>
