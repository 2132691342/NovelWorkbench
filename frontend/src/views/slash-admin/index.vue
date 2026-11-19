<script setup lang="ts">

// 斜杠技能页：内置与自定义技能列表及提示词模板编辑。
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { get, isApiNotReadyError, post } from '@/api/http'
import type { NWSlashCommandItemVo, NWSlashCommandSaveReqVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

// 分类编码选项（对齐后端 category_code 枚举）
const NW_CATEGORY_CODE_LIST = ['write', 'revise', 'outline', 'setting', 'other'] as const

interface NWSlashCommandFormState {
  command_code: string
  command_name: string
  category_code: string
  description_text: string
  prompt_template: string
  enabled_flag: boolean
  sort_order: number
}

const commandList = ref<NWSlashCommandItemVo[]>([])
const listLoading = ref(false)
const apiNotReady = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const deletingKey = ref('')
const editingCommandUlid = ref('')
const formRef = ref<FormInstance>()

const formState = reactive<NWSlashCommandFormState>({
  command_code: '',
  command_name: '',
  category_code: 'write',
  description_text: '',
  prompt_template: '',
  enabled_flag: true,
  sort_order: 0,
})

const formRules = computed<FormRules>(() => ({
  command_code: [{ required: true, message: t('slashAdmin.ruleCommandCodeRequired'), trigger: 'blur' }],
  command_name: [{ required: true, message: t('slashAdmin.ruleCommandNameRequired'), trigger: 'blur' }],
  prompt_template: [{ required: true, message: t('slashAdmin.rulePromptRequired'), trigger: 'blur' }],
}))

/** 分类编码转展示文案 */
function categoryLabel(categoryCode: string): string {
  switch (categoryCode) {
    case 'write':
      return t('slashAdmin.categoryWrite')
    case 'revise':
      return t('slashAdmin.categoryRevise')
    case 'outline':
      return t('slashAdmin.categoryOutline')
    case 'setting':
      return t('slashAdmin.categorySetting')
    default:
      return t('slashAdmin.categoryOther')
  }
}

/** 加载斜杠命令列表（接口未就绪 404 时友好提示，不弹窗不白屏） */
async function loadCommandList(): Promise<void> {
  listLoading.value = true
  try {
    const commandData = await get<NWSlashCommandItemVo[]>('/slash/list', { silent_error: true })
    commandList.value = Array.isArray(commandData) ? commandData : []
    apiNotReady.value = false
  } catch (error) {
    commandList.value = []
    apiNotReady.value = isApiNotReadyError(error)
  } finally {
    listLoading.value = false
  }
}

/** 重置弹窗表单 */
function resetFormState(): void {
  formState.command_code = ''
  formState.command_name = ''
  formState.category_code = 'write'
  formState.description_text = ''
  formState.prompt_template = ''
  formState.enabled_flag = true
  formState.sort_order = 0
  editingCommandUlid.value = ''
}

function handleCreateCommand(): void {
  resetFormState()
  dialogVisible.value = true
}

function handleEditCommand(commandItem: NWSlashCommandItemVo): void {
  resetFormState()
  editingCommandUlid.value = commandItem.command_ulid
  formState.command_code = commandItem.command_code
  formState.command_name = commandItem.command_name
  formState.category_code = commandItem.category_code
  formState.description_text = commandItem.description_text
  formState.prompt_template = commandItem.prompt_template
  formState.enabled_flag = commandItem.enabled_flag
  formState.sort_order = commandItem.sort_order
  dialogVisible.value = true
}

/** 保存（新建/编辑共用） */
async function handleSaveCommand(): Promise<void> {
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
    const saveReqVo: NWSlashCommandSaveReqVo = {
      command_code: formState.command_code,
      command_name: formState.command_name,
      category_code: formState.category_code,
      description_text: formState.description_text,
      prompt_template: formState.prompt_template,
      enabled_flag: formState.enabled_flag,
      sort_order: formState.sort_order,
    }
    if (editingCommandUlid.value) {
      saveReqVo.command_ulid = editingCommandUlid.value
    }
    await post<null>('/slash/save', saveReqVo)
    ElMessage.success(t('slashAdmin.saveSuccess'))
    dialogVisible.value = false
    await loadCommandList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    saving.value = false
  }
}

/** 删除（内置命令不可删） */
async function handleDeleteCommand(commandItem: NWSlashCommandItemVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('slashAdmin.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  deletingKey.value = commandItem.command_ulid
  try {
    await post<null>('/slash/delete', { command_ulid: commandItem.command_ulid })
    ElMessage.success(t('slashAdmin.deleteSuccess'))
    await loadCommandList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    deletingKey.value = ''
  }
}

onMounted(() => {
  void loadCommandList()
})
</script>

<template>
  <div class="slash-admin-page">
    <NWCard>
      <template #header>
        <h3 class="nw-card__title">{{ t('slashAdmin.title') }}</h3>
        <NWButton type="primary" @click="handleCreateCommand">{{ t('common.create') }}</NWButton>
      </template>

      <NWLoading v-if="listLoading" />
      <el-table v-else :data="commandList">
        <el-table-column prop="command_code" :label="t('slashAdmin.commandCode')" width="140" />
        <el-table-column prop="command_name" :label="t('slashAdmin.commandName')" width="150" />
        <el-table-column :label="t('slashAdmin.categoryCode')" width="100">
          <template #default="{ row }">
            <el-tag>{{ categoryLabel(row.category_code) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description_text" :label="t('slashAdmin.descriptionText')" min-width="200" />
        <el-table-column :label="t('slashAdmin.enabledFlag')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled_flag ? 'success' : 'info'">
              {{ row.enabled_flag ? t('slashAdmin.enabledTag') : t('slashAdmin.disabledTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('slashAdmin.builtinFlag')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.builtin_flag ? 'warning' : 'primary'">
              {{ row.builtin_flag ? t('slashAdmin.builtinTag') : t('slashAdmin.customTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="sort_order" :label="t('slashAdmin.sortOrder')" width="90" />
        <el-table-column :label="t('common.action')" width="180">
          <template #default="{ row }">
            <NWButton type="default" size="small" @click="handleEditCommand(row)">
              {{ t('common.edit') }}
            </NWButton>
            <NWButton
              type="danger"
              size="small"
              :disabled="row.builtin_flag"
              :loading="deletingKey === row.command_ulid"
              @click="handleDeleteCommand(row)"
            >
              {{ t('common.delete') }}
            </NWButton>
          </template>
        </el-table-column>
        <template #empty>
          <NWEmpty
            :description_text="apiNotReady ? t('slashAdmin.apiNotReady') : t('slashAdmin.emptyTip')"
          />
        </template>
      </el-table>
    </NWCard>

    <el-dialog
      v-model="dialogVisible"
      :title="editingCommandUlid ? t('slashAdmin.editTitle') : t('slashAdmin.createTitle')"
      width="640px"
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <el-form-item prop="command_code" :label="t('slashAdmin.commandCode')">
          <el-input v-model="formState.command_code" :disabled="Boolean(editingCommandUlid)" />
        </el-form-item>

        <el-form-item prop="command_name" :label="t('slashAdmin.commandName')">
          <el-input v-model="formState.command_name" />
        </el-form-item>

        <el-form-item prop="category_code" :label="t('slashAdmin.categoryCode')">
          <el-select v-model="formState.category_code" class="slash-admin-page__select">
            <el-option
              v-for="categoryCode in NW_CATEGORY_CODE_LIST"
              :key="categoryCode"
              :value="categoryCode"
              :label="categoryLabel(categoryCode)"
            />
          </el-select>
        </el-form-item>

        <el-form-item prop="description_text" :label="t('slashAdmin.descriptionText')">
          <el-input v-model="formState.description_text" type="textarea" :rows="2" />
        </el-form-item>

        <el-form-item prop="prompt_template" :label="t('slashAdmin.promptTemplate')">
          <el-input v-model="formState.prompt_template" type="textarea" :rows="6" />
        </el-form-item>

        <el-form-item prop="enabled_flag" :label="t('slashAdmin.enabledFlag')">
          <el-switch v-model="formState.enabled_flag" />
        </el-form-item>

        <el-form-item prop="sort_order" :label="t('slashAdmin.sortOrder')">
          <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>

      <template #footer>
        <NWButton type="default" @click="dialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="saving" @click="handleSaveCommand">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.slash-admin-page__select {
  width: 100%;
}
</style>
