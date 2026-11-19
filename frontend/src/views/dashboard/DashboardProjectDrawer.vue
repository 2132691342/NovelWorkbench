<!-- 仪表盘项目抽屉：新建与编辑共用，保存后通知父级刷新。 -->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { createProject, updateProject } from '@/api/project'
import type { NWProjectVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import { useProjectStore } from '@/stores/project'

const props = defineProps<{
  visible: boolean
  project: NWProjectVo | null
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  saved: []
}>()

const { t } = useI18n()
const projectStore = useProjectStore()
const formRef = ref<FormInstance>()
const savingFlag = ref(false)

const genreOptions = computed(() => [
  { value: 'fantasy', label: t('dashboard.genreFantasy') },
  { value: 'scifi', label: t('dashboard.genreScifi') },
  { value: 'romance', label: t('dashboard.genreRomance') },
  { value: 'suspense', label: t('dashboard.genreSuspense') },
  { value: 'wuxia', label: t('dashboard.genreWuxia') },
  { value: 'urban', label: t('dashboard.genreUrban') },
  { value: 'history', label: t('dashboard.genreHistory') },
])

const formState = reactive({
  project_name: '',
  genre_code: '',
  style_text: '',
  synopsis_text: '',
  project_status: 'draft',
})

const editingFlag = computed(() => Boolean(props.project))

const formRules = computed<FormRules>(() => ({
  project_name: [
    { required: true, message: t('dashboard.ruleProjectNameRequired'), trigger: 'blur' },
    { max: 128, message: t('dashboard.ruleProjectNameMax'), trigger: 'blur' },
  ],
}))

watch(
  () => props.visible,
  (visibleFlag) => {
    if (!visibleFlag) return
    formState.project_name = props.project?.project_name ?? ''
    formState.genre_code = props.project?.genre_code ?? ''
    formState.style_text = props.project?.style_text ?? ''
    formState.synopsis_text = props.project?.synopsis_text ?? ''
    formState.project_status = props.project?.project_status ?? 'draft'
    formRef.value?.clearValidate()
  },
)

async function handleSubmit(): Promise<void> {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  savingFlag.value = true
  try {
    if (props.project) {
      await updateProject({
        project_ulid: props.project.project_ulid,
        project_name: formState.project_name,
        genre_code: formState.genre_code,
        style_text: formState.style_text,
        synopsis_text: formState.synopsis_text,
        project_status: formState.project_status,
      })
      ElMessage.success(t('dashboard.updateSuccess'))
    } else {
      const createdProject = await createProject({
        project_name: formState.project_name,
        genre_code: formState.genre_code || undefined,
        style_text: formState.style_text || undefined,
        synopsis_text: formState.synopsis_text || undefined,
      })
      ElMessage.success(t('dashboard.createSuccess'))
      projectStore.setCurrentProject(createdProject.project_ulid)
    }
    emit('update:visible', false)
    emit('saved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingFlag.value = false
  }
}
</script>

<template>
  <el-drawer
    :model-value="visible"
    :title="editingFlag ? t('dashboard.editProjectTitle') : t('dashboard.createProjectTitle')"
    size="480px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
      <el-form-item :label="t('dashboard.projectName')" prop="project_name">
        <el-input
          v-model="formState.project_name"
          maxlength="128"
          show-word-limit
          :placeholder="t('dashboard.projectNamePlaceholder')"
        />
      </el-form-item>
      <div class="project-drawer__row">
        <el-form-item :label="t('dashboard.genreCode')" prop="genre_code">
          <el-select v-model="formState.genre_code" clearable class="project-drawer__full">
            <el-option
              v-for="genreOption in genreOptions"
              :key="genreOption.value"
              :label="genreOption.label"
              :value="genreOption.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="editingFlag" :label="t('dashboard.projectStatus')">
          <el-select v-model="formState.project_status" class="project-drawer__full">
            <el-option value="draft" :label="t('dashboard.statusDraft')" />
            <el-option value="serial" :label="t('dashboard.statusSerial')" />
            <el-option value="completed" :label="t('dashboard.statusCompleted')" />
          </el-select>
        </el-form-item>
      </div>
      <el-form-item :label="t('dashboard.styleText')" prop="style_text">
        <el-input
          v-model="formState.style_text"
          type="textarea"
          :rows="4"
          maxlength="1024"
          show-word-limit
          :placeholder="t('dashboard.stylePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('dashboard.synopsisText')" prop="synopsis_text">
        <el-input
          v-model="formState.synopsis_text"
          type="textarea"
          :rows="5"
          maxlength="4096"
          show-word-limit
          :placeholder="t('dashboard.synopsisPlaceholder')"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <div class="project-drawer__footer">
        <NWButton @click="emit('update:visible', false)">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="savingFlag" @click="handleSubmit">
          {{ editingFlag ? t('common.save') : t('dashboard.createProject') }}
        </NWButton>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.project-drawer__row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.project-drawer__full {
  width: 100%;
}

.project-drawer__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
