<!--
  技能管理页：作用域切换（全局 / 当前项目）+ 分类与关键词筛选 + 技能表格 + 编辑抽屉。
  抽屉内含提示词模板、变量声明与参考资料面板（按关键词命中注入上下文）。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { deleteSkill, listSkills, saveSkill, type NWSkillSaveReqVo } from '@/api/skill'
import type { NWSkillVariableVo, NWSkillVo } from '@/api/contracts'
import { useProjectStore } from '@/stores/project'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import SkillReferencePanel from './SkillReferencePanel.vue'

const { t } = useI18n()
const projectStore = useProjectStore()

/** 分类编码选项（对齐后端 SkillCategory*） */
const NW_CATEGORY_CODE_LIST = ['write', 'revise', 'outline', 'setting', 'review', 'other'] as const
/** 作用域选项 */
const NW_SCOPE_LIST = ['global', 'project'] as const

const skillList = ref<NWSkillVo[]>([])
const listLoading = ref(false)
const scopeFilter = ref<'global' | 'project'>('project')
const categoryFilter = ref('')
const keywordText = ref('')
const enabledOnlyFilter = ref(false)

const drawerVisible = ref(false)
const saving = ref(false)
const deletingKey = ref('')
const formRef = ref<FormInstance>()

/** 技能表单状态（variable_list 直接绑定数组） */
const formState = reactive({
  skill_ulid: '',
  skill_code: '',
  skill_name: '',
  scope_code: 'global' as string,
  project_ulid: '',
  category_code: 'write',
  description_text: '',
  prompt_template: '',
  variable_list: [] as NWSkillVariableVo[],
  enabled_flag: true,
  sort_order: 0,
})

const formRules = computed<FormRules>(() => ({
  skill_code: [{ required: true, message: t('skills.ruleCodeRequired'), trigger: 'blur' }],
  skill_name: [{ required: true, message: t('skills.ruleNameRequired'), trigger: 'blur' }],
  prompt_template: [{ required: true, message: t('skills.rulePromptRequired'), trigger: 'blur' }],
}))

/** 当前查询用的项目 ULID（项目视图下列出「项目技能 + 全局技能」） */
const queryProjectUlid = computed(() =>
  scopeFilter.value === 'project' ? projectStore.current_project_ulid : '',
)

function categoryLabel(categoryCode: string): string {
  switch (categoryCode) {
    case 'write':
      return t('skills.categoryWrite')
    case 'revise':
      return t('skills.categoryRevise')
    case 'outline':
      return t('skills.categoryOutline')
    case 'setting':
      return t('skills.categorySetting')
    case 'review':
      return t('skills.categoryReview')
    default:
      return t('skills.categoryOther')
  }
}

function scopeLabel(scopeCode: string): string {
  return scopeCode === 'project' ? t('skills.scopeProject') : t('skills.scopeGlobal')
}

async function loadSkillList(): Promise<void> {
  listLoading.value = true
  try {
    skillList.value = await listSkills({
      project_ulid: queryProjectUlid.value || undefined,
      scope_code: queryProjectUlid.value ? undefined : scopeFilter.value,
      category_code: categoryFilter.value || undefined,
      keyword_text: keywordText.value.trim() || undefined,
      enabled_only: enabledOnlyFilter.value,
    })
  } catch {
    // 失败提示由 http 层统一弹出，保留当前列表
  } finally {
    listLoading.value = false
  }
}

/** 切换作用域：项目视图需要已选项目 */
function handleScopeChange(scopeCode: 'global' | 'project'): void {
  if (scopeCode === 'project' && !projectStore.current_project_ulid) {
    ElMessage.warning(t('skills.needProject'))
    return
  }
  scopeFilter.value = scopeCode
  void loadSkillList()
}

function resetFormState(): void {
  formState.skill_ulid = ''
  formState.skill_code = ''
  formState.skill_name = ''
  formState.scope_code = scopeFilter.value
  formState.project_ulid = scopeFilter.value === 'project' ? projectStore.current_project_ulid : ''
  formState.category_code = categoryFilter.value || 'write'
  formState.description_text = ''
  formState.prompt_template = ''
  formState.variable_list = []
  formState.enabled_flag = true
  formState.sort_order = 0
}

/** 新建：项目视图默认创建项目技能，全局视图创建全局技能 */
function handleCreateSkill(): void {
  resetFormState()
  if (formState.scope_code === 'project' && !formState.project_ulid) {
    ElMessage.warning(t('skills.needProject'))
    return
  }
  drawerVisible.value = true
}

function handleEditSkill(skillItem: NWSkillVo): void {
  resetFormState()
  formState.skill_ulid = skillItem.skill_ulid
  formState.skill_code = skillItem.skill_code
  formState.skill_name = skillItem.skill_name
  formState.scope_code = skillItem.scope_code
  formState.project_ulid = skillItem.project_ulid
  formState.category_code = skillItem.category_code || 'other'
  formState.description_text = skillItem.description_text
  formState.prompt_template = skillItem.prompt_template
  formState.variable_list = (skillItem.variable_list ?? []).map((variableItem) => ({ ...variableItem }))
  formState.enabled_flag = skillItem.enabled_flag
  formState.sort_order = skillItem.sort_order
  drawerVisible.value = true
}

async function handleSaveSkill(): Promise<void> {
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
    const saveReqVo: NWSkillSaveReqVo = {
      skill_code: formState.skill_code.trim(),
      skill_name: formState.skill_name.trim(),
      scope_code: formState.scope_code,
      category_code: formState.category_code,
      description_text: formState.description_text,
      prompt_template: formState.prompt_template,
      variable_list: formState.variable_list.filter((variableItem) => variableItem.variable_name.trim()),
      enabled_flag: formState.enabled_flag,
      sort_order: formState.sort_order,
    }
    if (formState.scope_code === 'project') {
      saveReqVo.project_ulid = formState.project_ulid
    }
    if (formState.skill_ulid) {
      saveReqVo.skill_ulid = formState.skill_ulid
    }
    const savedSkill = await saveSkill(saveReqVo)
    ElMessage.success(t('skills.saveSuccess'))
    formState.skill_ulid = savedSkill.skill_ulid
    await loadSkillList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    saving.value = false
  }
}

async function handleDeleteSkill(skillItem: NWSkillVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('skills.deleteConfirm', { name: skillItem.skill_name }),
      t('common.confirm'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' },
    )
  } catch {
    return
  }
  deletingKey.value = skillItem.skill_ulid
  try {
    await deleteSkill(skillItem.skill_ulid)
    ElMessage.success(t('skills.deleteSuccess'))
    if (formState.skill_ulid === skillItem.skill_ulid) {
      drawerVisible.value = false
    }
    await loadSkillList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    deletingKey.value = ''
  }
}

/** 变量声明：增删行 */
function handleAddVariable(): void {
  formState.variable_list.push({
    variable_name: '',
    variable_label: '',
    default_value: '',
    required_flag: false,
    description_text: '',
  })
}

function handleRemoveVariable(variableIndex: number): void {
  formState.variable_list.splice(variableIndex, 1)
}

function handleSearch(): void {
  void loadSkillList()
}

function handleResetFilters(): void {
  categoryFilter.value = ''
  keywordText.value = ''
  enabledOnlyFilter.value = false
  void loadSkillList()
}

onMounted(() => {
  void loadSkillList()
})
</script>

<template>
  <div class="nw-page skills-page">
    <NWPageHeader
      :title_text="t('skills.title')"
      :subtitle_text="t('skills.hint')"
      icon_code="wand"
    >
      <template #actions>
        <NWButton type="primary" @click="handleCreateSkill">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('common.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <NWCard :hoverable="false">
      <template #header>
        <div class="skills-page__header-actions">
          <el-radio-group :model-value="scopeFilter" @change="handleScopeChange">
            <el-radio-button value="project">{{ t('skills.scopeProject') }}</el-radio-button>
            <el-radio-button value="global">{{ t('skills.scopeGlobal') }}</el-radio-button>
          </el-radio-group>
          <el-input
            v-model="keywordText"
            class="skills-page__search"
            :placeholder="t('skills.keywordPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          />
          <el-select v-model="categoryFilter" class="skills-page__filter" :placeholder="t('skills.categoryAll')" clearable>
            <el-option
              v-for="categoryCode in NW_CATEGORY_CODE_LIST"
              :key="categoryCode"
              :value="categoryCode"
              :label="categoryLabel(categoryCode)"
            />
          </el-select>
          <el-checkbox v-model="enabledOnlyFilter" @change="handleSearch">{{ t('skills.enabledOnly') }}</el-checkbox>
          <NWButton @click="handleSearch">{{ t('admin.search') }}</NWButton>
          <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
        </div>
      </template>


      <NWLoading v-if="listLoading" />
      <el-table v-else :data="skillList">
        <el-table-column :label="t('skills.skillName')" min-width="170">
          <template #default="{ row }">
            <div class="skills-page__name-cell">
              <code class="skills-page__code">/{{ row.skill_code }}</code>
              <span>{{ row.skill_name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('skills.scope')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.scope_code === 'project' ? 'success' : 'info'">
              {{ scopeLabel(row.scope_code) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('skills.category')" width="96">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ categoryLabel(row.category_code) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description_text" :label="t('skills.description')" min-width="200" show-overflow-tooltip />
        <el-table-column :label="t('skills.variableCount')" width="80" align="center">
          <template #default="{ row }">{{ row.variable_list?.length ?? 0 }}</template>
        </el-table-column>
        <el-table-column :label="t('skills.refCount')" width="80" align="center">
          <template #default="{ row }">{{ row.ref_count }}</template>
        </el-table-column>
        <el-table-column :label="t('skills.status')" width="190">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled_flag ? 'success' : 'info'">
              {{ row.enabled_flag ? t('skills.enabledTag') : t('skills.disabledTag') }}
            </el-tag>
            <el-tag v-if="row.builtin_flag" size="small" type="warning" class="skills-page__tag-gap">
              {{ t('skills.builtinTag') }}
            </el-tag>
            <el-tag v-if="row.overridden_flag" size="small" type="warning" effect="plain" class="skills-page__tag-gap">
              {{ t('skills.overriddenTag') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.action')" width="160" fixed="right">
          <template #default="{ row }">
            <NWButton size="small" @click="handleEditSkill(row)">{{ t('common.edit') }}</NWButton>
            <NWButton
              type="danger-text"
              size="small"
              :disabled="row.builtin_flag"
              :loading="deletingKey === row.skill_ulid"
              @click="handleDeleteSkill(row)"
            >
              {{ t('common.delete') }}
            </NWButton>
          </template>
        </el-table-column>
        <template #empty>
          <NWEmpty :description_text="t('skills.emptyTip')" />
        </template>
      </el-table>
    </NWCard>

    <!-- 技能编辑抽屉 -->
    <el-drawer
      v-model="drawerVisible"
      :title="formState.skill_ulid ? t('skills.editTitle') : t('skills.createTitle')"
      size="720px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <div class="skills-page__grid">
          <el-form-item prop="skill_name" :label="t('skills.skillName')">
            <el-input v-model="formState.skill_name" :maxlength="64" show-word-limit />
          </el-form-item>
          <el-form-item prop="skill_code" :label="t('skills.skillCode')">
            <el-input v-model="formState.skill_code" :maxlength="32" :disabled="Boolean(formState.skill_ulid)" />
          </el-form-item>
        </div>

        <div class="skills-page__grid">
          <el-form-item :label="t('skills.scope')">
            <el-select v-model="formState.scope_code" class="skills-page__full" :disabled="Boolean(formState.skill_ulid)">
              <el-option v-for="scopeCode in NW_SCOPE_LIST" :key="scopeCode" :value="scopeCode" :label="scopeLabel(scopeCode)" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('skills.category')">
            <el-select v-model="formState.category_code" class="skills-page__full">
              <el-option
                v-for="categoryCode in NW_CATEGORY_CODE_LIST"
                :key="categoryCode"
                :value="categoryCode"
                :label="categoryLabel(categoryCode)"
              />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item :label="t('skills.description')">
          <el-input v-model="formState.description_text" :maxlength="256" show-word-limit />
        </el-form-item>

        <el-form-item prop="prompt_template" :label="t('skills.promptTemplate')">
          <el-input
            v-model="formState.prompt_template"
            type="textarea"
            :rows="9"
            :maxlength="8000"
            show-word-limit
            class="skills-page__template"
          />
          <p class="skills-page__field-hint">{{ t('skills.promptTemplateHint') }}</p>
        </el-form-item>

        <!-- 变量声明 -->
        <div class="skills-page__section">
          <div class="skills-page__section-head">
            <span class="skills-page__section-title">{{ t('skills.variableTitle') }}</span>
            <NWButton size="small" @click="handleAddVariable">{{ t('skills.variableAdd') }}</NWButton>
          </div>
          <p class="skills-page__field-hint">{{ t('skills.variableHint') }}</p>
          <div v-if="formState.variable_list.length" class="skills-page__variable-list">
            <div
              v-for="(variableItem, variableIndex) in formState.variable_list"
              :key="variableIndex"
              class="skills-page__variable-row"
            >
              <el-input
                v-model="variableItem.variable_name"
                :placeholder="t('skills.variableName')"
                class="skills-page__variable-name"
              />
              <el-input v-model="variableItem.variable_label" :placeholder="t('skills.variableLabel')" />
              <el-input v-model="variableItem.default_value" :placeholder="t('skills.variableDefault')" />
              <el-switch v-model="variableItem.required_flag" :active-text="t('skills.variableRequired')" />
              <NWButton type="danger-text" size="small" @click="handleRemoveVariable(variableIndex)">
                {{ t('common.delete') }}
              </NWButton>
            </div>
          </div>
          <p v-else class="skills-page__muted">{{ t('skills.variableEmpty') }}</p>
        </div>

        <!-- 参考资料：未保存技能前禁用 -->
        <div class="skills-page__section">
          <SkillReferencePanel
            :skill_ulid="formState.skill_ulid"
            :disabled_flag="!formState.skill_ulid"
            @changed="loadSkillList"
          />
        </div>

        <div class="skills-page__grid">
          <el-form-item :label="t('skills.enabledFlag')">
            <el-switch v-model="formState.enabled_flag" />
          </el-form-item>
          <el-form-item :label="t('skills.sortOrder')">
            <el-input-number v-model="formState.sort_order" :min="0" :max="9999" class="skills-page__full" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="skills-page__drawer-footer">
          <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
          <NWButton type="primary" :loading="saving" @click="handleSaveSkill">{{ t('common.save') }}</NWButton>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped src="./skills.css"></style>
