<!--
  角色新建/编辑抽屉：CharacterREQ 全字段表单，含必填/长度校验。
-->
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { saveCharacter, type NWCharacterVo } from '@/api/character'
import NWButton from '@/components/NWButton.vue'

const props = defineProps<{
  visible: boolean
  projectUlid: string
  character: NWCharacterVo | null
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  saved: [character: NWCharacterVo]
}>()

const { t } = useI18n()

/** 角色类型选项（protagonist/antagonist/supporting/minor） */
const roleTypeOptions = ['protagonist', 'antagonist', 'supporting', 'minor']

const formRef = ref<FormInstance | null>(null)
const saving = ref(false)

const formModel = reactive({
  character_name: '',
  alias_text: '',
  role_type_code: '',
  personality_text: '',
  background_text: '',
  appearance_text: '',
  goal_text: '',
})

const formRules: FormRules = {
  character_name: [
    { required: true, message: () => t('character.ruleNameRequired'), trigger: 'blur' },
  ],
}

/** 角色类型展示文案 */
function roleTypeText(roleTypeCode: string): string {
  switch (roleTypeCode) {
    case 'protagonist':
      return t('character.roleProtagonist')
    case 'antagonist':
      return t('character.roleAntagonist')
    case 'supporting':
      return t('character.roleSupporting')
    case 'minor':
      return t('character.roleMinor')
    default:
      return roleTypeCode
  }
}

/** 打开抽屉时按编辑/新建回填表单 */
function resetForm(): void {
  const character = props.character
  formModel.character_name = character?.character_name ?? ''
  formModel.alias_text = character?.alias_text ?? ''
  formModel.role_type_code = character?.role_type_code ?? ''
  formModel.personality_text = character?.personality_text ?? ''
  formModel.background_text = character?.background_text ?? ''
  formModel.appearance_text = character?.appearance_text ?? ''
  formModel.goal_text = character?.goal_text ?? ''
  formRef.value?.clearValidate()
}

/** 提交保存（有 character_ulid 为更新） */
async function handleSave(): Promise<void> {
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
    const savedVo = await saveCharacter({
      character_ulid: props.character?.character_ulid,
      project_ulid: props.projectUlid,
      character_name: formModel.character_name,
      alias_text: formModel.alias_text,
      role_type_code: formModel.role_type_code || undefined,
      personality_text: formModel.personality_text,
      background_text: formModel.background_text,
      appearance_text: formModel.appearance_text,
      goal_text: formModel.goal_text,
    })
    ElMessage.success(t('character.saveSuccess'))
    emit('saved', savedVo)
    emit('update:visible', false)
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    saving.value = false
  }
}

function handleClose(): void {
  emit('update:visible', false)
}

watch(
  () => props.visible,
  (visibleFlag) => {
    if (visibleFlag) {
      resetForm()
    }
  },
)
</script>

<template>
  <el-drawer
    :model-value="visible"
    :title="character ? t('character.editTitle') : t('character.createTitle')"
    size="480px"
    @close="handleClose"
    @update:model-value="handleClose"
  >
    <el-form ref="formRef" :model="formModel" :rules="formRules" label-position="top">
      <el-form-item :label="t('character.name')" prop="character_name">
        <el-input
          v-model="formModel.character_name"
          maxlength="64"
          :placeholder="t('character.namePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('character.alias')">
        <el-input
          v-model="formModel.alias_text"
          maxlength="128"
          :placeholder="t('character.aliasPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('character.roleType')">
        <el-select
          v-model="formModel.role_type_code"
          clearable
          :placeholder="t('character.roleTypePlaceholder')"
        >
          <el-option
            v-for="roleType in roleTypeOptions"
            :key="roleType"
            :label="roleTypeText(roleType)"
            :value="roleType"
          />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('character.personality')">
        <el-input
          v-model="formModel.personality_text"
          type="textarea"
          :rows="3"
          maxlength="2048"
          :placeholder="t('character.personalityPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('character.background')">
        <el-input
          v-model="formModel.background_text"
          type="textarea"
          :rows="3"
          maxlength="4096"
          :placeholder="t('character.backgroundPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('character.appearance')">
        <el-input
          v-model="formModel.appearance_text"
          type="textarea"
          :rows="3"
          maxlength="2048"
          :placeholder="t('character.appearancePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('character.goal')">
        <el-input
          v-model="formModel.goal_text"
          type="textarea"
          :rows="3"
          maxlength="2048"
          :placeholder="t('character.goalPlaceholder')"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <NWButton @click="handleClose">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="saving" @click="handleSave">
        {{ t('common.save') }}
      </NWButton>
    </template>
  </el-drawer>
</template>
