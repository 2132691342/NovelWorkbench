<!-- 事实与伏笔的表单弹窗：负责校验与保存。 -->
<script setup lang="ts">
  import { reactive, ref } from 'vue'
  import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
  import {
    saveFact,
    saveHook,
    type NWForeshadowHookVo,
    type NWHookStatusCode,
    type NWImportanceCode,
    type NWTemporalFactVo
  } from '@/api/continuity'
  import type { NWChapterBriefVo } from '@/api/contracts'

  const props = defineProps<{
    projectUlid: string
    chapterList: NWChapterBriefVo[]
    hookStatusOptions: Array<{ value: NWHookStatusCode; label: string }>
    importanceOptions: Array<{ value: NWImportanceCode; label: string }>
  }>()

  const emit = defineEmits<{ saved: [] }>()

  // ---------------- 事实 ----------------
  const factVisible = ref(false)
  const factFormRef = ref<FormInstance>()
  const factSaving = ref(false)
  const factForm = reactive({
    fact_ulid: '',
    subject_text: '',
    predicate_code: 'is',
    object_text: '',
    source_chapter_ulid: '',
    source_quote_text: '',
    note_text: ''
  })
  const factRules: FormRules = {
    subject_text: [{ required: true, message: '请填写主体', trigger: 'blur' }],
    predicate_code: [{ required: true, message: '请填写谓词', trigger: 'blur' }],
    source_chapter_ulid: [{ required: true, message: '请选择来源章节', trigger: 'change' }]
  }

  const openFact = (fact?: NWTemporalFactVo) => {
    factForm.fact_ulid = fact?.fact_ulid ?? ''
    factForm.subject_text = fact?.subject_text ?? ''
    factForm.predicate_code = fact?.predicate_code ?? 'is'
    factForm.object_text = fact?.object_text ?? ''
    factForm.source_chapter_ulid =
      fact?.source_chapter_ulid ?? props.chapterList[0]?.chapter_ulid ?? ''
    factForm.source_quote_text = fact?.source_quote_text ?? ''
    factForm.note_text = fact?.note_text ?? ''
    factVisible.value = true
  }

  const submitFact = async () => {
    const valid = await factFormRef.value?.validate().catch(() => false)
    if (!valid) return
    factSaving.value = true
    try {
      await saveFact({
        fact_ulid: factForm.fact_ulid || undefined,
        project_ulid: props.projectUlid,
        subject_text: factForm.subject_text,
        predicate_code: factForm.predicate_code,
        object_text: factForm.object_text,
        source_chapter_ulid: factForm.source_chapter_ulid,
        source_quote_text: factForm.source_quote_text,
        note_text: factForm.note_text
      })
      ElMessage.success('事实已保存')
      factVisible.value = false
      emit('saved')
    } finally {
      factSaving.value = false
    }
  }

  // ---------------- 伏笔 ----------------
  const hookVisible = ref(false)
  const hookFormRef = ref<FormInstance>()
  const hookSaving = ref(false)
  const hookForm = reactive({
    hook_ulid: '',
    hook_status_code: 'planned' as NWHookStatusCode,
    description_text: '',
    expected_payoff_text: '',
    setup_chapter_ulid: '',
    payoff_chapter_ulid: '',
    importance_code: 'medium' as NWImportanceCode,
    note_text: ''
  })
  const hookRules: FormRules = {
    description_text: [{ required: true, message: '请填写伏笔内容', trigger: 'blur' }]
  }

  const openHook = (hook?: NWForeshadowHookVo) => {
    hookForm.hook_ulid = hook?.hook_ulid ?? ''
    hookForm.hook_status_code = hook?.hook_status_code ?? 'planned'
    hookForm.description_text = hook?.description_text ?? ''
    hookForm.expected_payoff_text = hook?.expected_payoff_text ?? ''
    hookForm.setup_chapter_ulid = hook?.setup_chapter_ulid ?? ''
    hookForm.payoff_chapter_ulid = hook?.payoff_chapter_ulid ?? ''
    hookForm.importance_code = hook?.importance_code ?? 'medium'
    hookForm.note_text = hook?.note_text ?? ''
    hookVisible.value = true
  }

  const submitHook = async () => {
    const valid = await hookFormRef.value?.validate().catch(() => false)
    if (!valid) return
    hookSaving.value = true
    try {
      await saveHook({
        ...hookForm,
        hook_ulid: hookForm.hook_ulid || undefined,
        project_ulid: props.projectUlid
      })
      ElMessage.success('伏笔已保存')
      hookVisible.value = false
      emit('saved')
    } finally {
      hookSaving.value = false
    }
  }

  defineExpose({ openFact, openHook })
</script>

<template>
  <ElDialog v-model="factVisible" :title="factForm.fact_ulid ? '编辑事实' : '新建事实'" width="560px">
    <ElForm ref="factFormRef" :model="factForm" :rules="factRules" label-position="top">
      <div class="dialog-form__row">
        <ElFormItem label="主体" prop="subject_text">
          <ElInput v-model="factForm.subject_text" placeholder="人物 / 物品 / 地点" maxlength="80" />
        </ElFormItem>
        <ElFormItem label="谓词" prop="predicate_code">
          <ElInput v-model="factForm.predicate_code" placeholder="is / has / at" maxlength="32" />
        </ElFormItem>
      </div>
      <ElFormItem label="客体">
        <ElInput v-model="factForm.object_text" placeholder="事实内容" maxlength="200" />
      </ElFormItem>
      <ElFormItem label="来源章节" prop="source_chapter_ulid">
        <ElSelect v-model="factForm.source_chapter_ulid" class="dialog-form__select" filterable>
          <ElOption
            v-for="chapter in chapterList"
            :key="chapter.chapter_ulid"
            :label="chapter.chapter_name"
            :value="chapter.chapter_ulid"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="原文摘录">
        <ElInput v-model="factForm.source_quote_text" type="textarea" :rows="2" maxlength="500" />
      </ElFormItem>
      <ElFormItem label="备注">
        <ElInput v-model="factForm.note_text" maxlength="200" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="factVisible = false">取消</ElButton>
      <ElButton type="primary" :loading="factSaving" @click="submitFact">保存</ElButton>
    </template>
  </ElDialog>

  <ElDialog v-model="hookVisible" :title="hookForm.hook_ulid ? '编辑伏笔' : '新建伏笔'" width="560px">
    <ElForm ref="hookFormRef" :model="hookForm" :rules="hookRules" label-position="top">
      <ElFormItem label="伏笔内容" prop="description_text">
        <ElInput v-model="hookForm.description_text" type="textarea" :rows="2" maxlength="300" />
      </ElFormItem>
      <ElFormItem label="预期回收方式">
        <ElInput v-model="hookForm.expected_payoff_text" type="textarea" :rows="2" maxlength="300" />
      </ElFormItem>
      <div class="dialog-form__row">
        <ElFormItem label="状态">
          <ElSelect v-model="hookForm.hook_status_code" class="dialog-form__select">
            <ElOption
              v-for="item in hookStatusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="重要度">
          <ElSelect v-model="hookForm.importance_code" class="dialog-form__select">
            <ElOption
              v-for="item in importanceOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
      </div>
      <div class="dialog-form__row">
        <ElFormItem label="埋设章节">
          <ElSelect v-model="hookForm.setup_chapter_ulid" class="dialog-form__select" clearable filterable>
            <ElOption
              v-for="chapter in chapterList"
              :key="chapter.chapter_ulid"
              :label="chapter.chapter_name"
              :value="chapter.chapter_ulid"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="回收章节">
          <ElSelect v-model="hookForm.payoff_chapter_ulid" class="dialog-form__select" clearable filterable>
            <ElOption
              v-for="chapter in chapterList"
              :key="chapter.chapter_ulid"
              :label="chapter.chapter_name"
              :value="chapter.chapter_ulid"
            />
          </ElSelect>
        </ElFormItem>
      </div>
      <ElFormItem label="备注">
        <ElInput v-model="hookForm.note_text" maxlength="200" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="hookVisible = false">取消</ElButton>
      <ElButton type="primary" :loading="hookSaving" @click="submitHook">保存</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .dialog-form {
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
