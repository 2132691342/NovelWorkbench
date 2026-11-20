<!--
  提示词档案管理：系统提示词决定 AI 的身份与底线，与技能（单次任务指令）分工。
  全局档案所有项目可用，项目档案只对所属项目生效；内置档案可改正文但不可删。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { deletePromptProfile, listPromptProfiles, savePromptProfile } from '@/api/prompt'
import type { NWPromptProfileVo } from '@/api/contracts'
import { useProjectStore } from '@/stores/project'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWLoading from '@/components/NWLoading.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'

const { t } = useI18n()
const projectStore = useProjectStore()

const profileList = ref<NWPromptProfileVo[]>([])
const listLoading = ref(false)
const scopeFilter = ref<'project' | 'global'>('project')
const drawerVisible = ref(false)
const savingFlag = ref(false)
const formRef = ref<FormInstance>()

/** 档案表单状态 */
const formState = reactive({
  prompt_profile_ulid: '',
  prompt_profile_code: '',
  prompt_profile_name: '',
  scope_code: 'global',
  project_ulid: '',
  description_text: '',
  system_prompt_text: '',
  enabled_flag: true,
  sort_order: 0,
})

const formRules = computed<FormRules>(() => ({
  prompt_profile_code: [{ required: true, message: t('prompts.ruleCodeRequired'), trigger: 'blur' }],
  prompt_profile_name: [{ required: true, message: t('prompts.ruleNameRequired'), trigger: 'blur' }],
  system_prompt_text: [{ required: true, message: t('prompts.rulePromptRequired'), trigger: 'blur' }],
}))

/** 当前查询的项目 ULID（全局视图下不传，只看全局档案） */
const queryProjectUlid = computed(() =>
  scopeFilter.value === 'project' ? projectStore.current_project_ulid : '',
)

/** 编辑态判定 */
const editingFlag = computed(() => formState.prompt_profile_ulid !== '')

/** 加载档案列表 */
async function loadProfileList(): Promise<void> {
  listLoading.value = true
  try {
    const listData = await listPromptProfiles({ project_ulid: queryProjectUlid.value || undefined })
    profileList.value = Array.isArray(listData) ? listData : []
  } catch {
    profileList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 新建档案 */
function handleCreate(): void {
  formState.prompt_profile_ulid = ''
  formState.prompt_profile_code = ''
  formState.prompt_profile_name = ''
  formState.scope_code = scopeFilter.value
  formState.project_ulid = queryProjectUlid.value
  formState.description_text = ''
  formState.system_prompt_text = ''
  formState.enabled_flag = true
  formState.sort_order = (profileList.value.length + 1) * 10
  drawerVisible.value = true
  formRef.value?.clearValidate()
}

/** 编辑档案 */
function handleEdit(row: NWPromptProfileVo): void {
  formState.prompt_profile_ulid = row.prompt_profile_ulid
  formState.prompt_profile_code = row.prompt_profile_code
  formState.prompt_profile_name = row.prompt_profile_name
  formState.scope_code = row.scope_code
  formState.project_ulid = row.project_ulid
  formState.description_text = row.description_text
  formState.system_prompt_text = row.system_prompt_text
  formState.enabled_flag = row.enabled_flag
  formState.sort_order = row.sort_order
  drawerVisible.value = true
  formRef.value?.clearValidate()
}

/** 保存档案 */
async function handleSave(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  if (formState.scope_code === 'project' && !formState.project_ulid) {
    ElMessage.warning(t('prompts.projectRequired'))
    return
  }
  savingFlag.value = true
  try {
    await savePromptProfile({
      prompt_profile_ulid: formState.prompt_profile_ulid || undefined,
      prompt_profile_code: formState.prompt_profile_code,
      prompt_profile_name: formState.prompt_profile_name,
      scope_code: formState.scope_code,
      project_ulid: formState.scope_code === 'project' ? formState.project_ulid : undefined,
      description_text: formState.description_text,
      system_prompt_text: formState.system_prompt_text,
      enabled_flag: formState.enabled_flag,
      sort_order: formState.sort_order,
    })
    ElMessage.success(editingFlag.value ? t('prompts.updateSuccess') : t('prompts.createSuccess'))
    drawerVisible.value = false
    await loadProfileList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingFlag.value = false
  }
}

/** 删除档案（内置档案不可删，后端同样拦截） */
async function handleDelete(row: NWPromptProfileVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('prompts.deleteConfirm', { name: row.prompt_profile_name }),
      t('prompts.deleteTitle'),
      { type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel') },
    )
  } catch {
    return
  }
  try {
    await deletePromptProfile(row.prompt_profile_ulid)
    ElMessage.success(t('prompts.deleteSuccess'))
    await loadProfileList()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 切换作用域后重新查询 */
function handleScopeChange(): void {
  void loadProfileList()
}

onMounted(() => {
  void loadProfileList()
})
</script>

<template>
  <div class="nw-page prompts-page">
    <NWPageHeader :title_text="t('prompts.title')" :subtitle_text="t('prompts.hint')" icon_code="sparkles">
      <template #actions>
        <NWButton type="primary" @click="handleCreate">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('common.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <NWCard :hoverable="false">
      <template #header>
        <div class="prompts-page__filters">
          <el-radio-group :model-value="scopeFilter" @change="handleScopeChange">
            <el-radio-button value="project">{{ t('prompts.scopeProject') }}</el-radio-button>
            <el-radio-button value="global">{{ t('prompts.scopeGlobal') }}</el-radio-button>
          </el-radio-group>
          <span class="prompts-page__count">{{ t('prompts.countLabel', { count: profileList.length }) }}</span>
        </div>
      </template>

      <NWLoading v-if="listLoading" />
      <NWEmpty v-else-if="profileList.length === 0" :title_text="t('prompts.emptyTitle')" :description_text="t('prompts.emptyDesc')" />
      <el-table v-else :data="profileList">
        <el-table-column :label="t('prompts.profileName')" min-width="180">
          <template #default="{ row }">
            <div class="prompts-page__name-cell">
              <code class="prompts-page__code">{{ row.prompt_profile_code }}</code>
              <span>{{ row.prompt_profile_name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('prompts.scope')" width="96">
          <template #default="{ row }">
            <el-tag size="small" :type="row.scope_code === 'project' ? 'success' : 'info'">
              {{ row.scope_code === 'project' ? t('prompts.scopeProjectTag') : t('prompts.scopeGlobalTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description_text" :label="t('prompts.description')" min-width="220" show-overflow-tooltip />
        <el-table-column :label="t('prompts.promptLength')" width="96" align="center">
          <template #default="{ row }">{{ row.system_prompt_text.length }}</template>
        </el-table-column>
        <el-table-column :label="t('prompts.status')" width="150">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled_flag ? 'success' : 'info'">
              {{ row.enabled_flag ? t('prompts.enabledTag') : t('prompts.disabledTag') }}
            </el-tag>
            <el-tag v-if="row.builtin_flag" size="small" type="warning" class="prompts-page__tag-gap">
              {{ t('prompts.builtinTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.action')" width="150" fixed="right">
          <template #default="{ row }">
            <NWButton type="text" size="small" @click="handleEdit(row)">{{ t('common.edit') }}</NWButton>
            <NWButton
              type="danger-text"
              size="small"
              :disabled="row.builtin_flag"
              @click="handleDelete(row)"
            >
              {{ t('common.delete') }}
            </NWButton>
          </template>
        </el-table-column>
      </el-table>
    </NWCard>

    <el-drawer
      v-model="drawerVisible"
      :title="editingFlag ? t('prompts.editTitle') : t('prompts.createTitle')"
      size="620px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <el-form-item :label="t('prompts.profileCode')" prop="prompt_profile_code">
          <el-input
            v-model="formState.prompt_profile_code"
            maxlength="32"
            :disabled="editingFlag"
            :placeholder="t('prompts.profileCodePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('prompts.profileName')" prop="prompt_profile_name">
          <el-input v-model="formState.prompt_profile_name" maxlength="64" />
        </el-form-item>
        <el-form-item :label="t('prompts.description')">
          <el-input v-model="formState.description_text" maxlength="256" show-word-limit />
        </el-form-item>
        <el-form-item :label="t('prompts.systemPrompt')" prop="system_prompt_text">
          <el-input
            v-model="formState.system_prompt_text"
            type="textarea"
            :rows="16"
            maxlength="8000"
            show-word-limit
            :placeholder="t('prompts.systemPromptPlaceholder')"
          />
        </el-form-item>
        <div class="prompts-page__form-row">
          <el-form-item :label="t('prompts.sortOrder')">
            <el-input-number v-model="formState.sort_order" :min="0" :max="9999" :step="10" />
          </el-form-item>
          <el-form-item :label="t('prompts.status')">
            <el-switch v-model="formState.enabled_flag" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="savingFlag" @click="handleSave">{{ t('common.save') }}</NWButton>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped src="./prompts.css"></style>
