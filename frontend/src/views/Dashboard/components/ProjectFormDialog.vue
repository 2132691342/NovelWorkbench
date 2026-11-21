<!-- 项目编辑弹窗：新建与修改项目基础信息。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
  import { useI18n } from 'vue-i18n'
  import { createProject, updateProject } from '@/api/project'
  import type { NWProjectVo } from '@/api/contracts'

  const emit = defineEmits<{ saved: [] }>()

  /** 弹窗可见性：由组件内部管理，父组件通过 ref.open() 打开 */
  const visible = ref(false)

  const { t } = useI18n()

  const formRef = ref<FormInstance>()
  const editing = ref<NWProjectVo | null>(null)
  const saving = ref(false)
  const form = reactive({
    project_name: '',
    genre_code: '',
    style_text: '',
    synopsis_text: '',
    project_status: 'draft'
  })

  const title = computed(() => (editing.value ? '编辑项目' : '新建项目'))

  const genreOptions = [
    { value: '', label: '未分类' },
    { value: 'fantasy', label: '玄幻' },
    { value: 'scifi', label: '科幻' },
    { value: 'romance', label: '言情' },
    { value: 'suspense', label: '悬疑' },
    { value: 'wuxia', label: '武侠' },
    { value: 'urban', label: '都市' },
    { value: 'history', label: '历史' }
  ]

  const rules = computed<FormRules>(() => ({
    project_name: [{ required: true, message: t('common.required'), trigger: 'blur' }]
  }))

  /** 打开弹窗：传项目为编辑，不传为新建 */
  const open = (project?: NWProjectVo) => {
    editing.value = project ?? null
    form.project_name = project?.project_name ?? ''
    form.genre_code = project?.genre_code ?? ''
    form.style_text = project?.style_text ?? ''
    form.synopsis_text = project?.synopsis_text ?? ''
    form.project_status = project?.project_status ?? 'draft'
    visible.value = true
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      if (editing.value) {
        await updateProject({ project_ulid: editing.value.project_ulid, ...form })
      } else {
        await createProject({
          project_name: form.project_name,
          genre_code: form.genre_code,
          style_text: form.style_text,
          synopsis_text: form.synopsis_text
        })
      }
      ElMessage.success(t('common.saveSuccess'))
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
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="项目名称" prop="project_name">
        <ElInput v-model="form.project_name" maxlength="80" show-word-limit placeholder="例如：长夜行" />
      </ElFormItem>
      <div class="project-form__row">
        <ElFormItem label="题材">
          <ElSelect v-model="form.genre_code" class="project-form__select">
            <ElOption
              v-for="item in genreOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="editing" label="状态">
          <ElSelect v-model="form.project_status" class="project-form__select">
            <ElOption label="草稿" value="draft" />
            <ElOption label="连载中" value="serial" />
            <ElOption label="已完结" value="completed" />
          </ElSelect>
        </ElFormItem>
      </div>
      <ElFormItem label="文风要求">
        <ElInput
          v-model="form.style_text"
          type="textarea"
          :rows="2"
          maxlength="300"
          placeholder="例如：冷峻克制，短句为主，少形容词"
        />
      </ElFormItem>
      <ElFormItem label="简介">
        <ElInput v-model="form.synopsis_text" type="textarea" :rows="3" maxlength="500" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">{{ t('common.cancel') }}</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">{{ t('common.save') }}</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .project-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }
  }
</style>
