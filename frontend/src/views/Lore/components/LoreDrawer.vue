<!-- 设定编辑抽屉：维护世界观设定内容。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElOption,
    ElSelect
  } from 'element-plus'
  import { saveLore } from '@/api/lore'
  import type { NWLoreVo } from '@/api/lore'

  /** 设定类型选项（note 归创作笔记页，这里不提供） */
  const LORE_TYPE_OPTIONS = [
    { value: 'world', label: '世界规则' },
    { value: 'rule', label: '体系法则' },
    { value: 'item', label: '物品道具' },
    { value: 'place', label: '地点场景' },
    { value: 'event', label: '历史事件' }
  ]

  const props = defineProps<{ projectUlid: string }>()

  const emit = defineEmits<{ saved: [lore: NWLoreVo] }>()

  const visible = ref(false)
  const saving = ref(false)
  const editingUlid = ref('')

  const form = reactive({
    lore_name: '',
    lore_type_code: 'world',
    content_text: ''
  })

  const title = computed(() => (editingUlid.value ? '编辑设定' : '新建设定'))

  /** 打开抽屉：传 lore 为编辑，不传为新建 */
  const open = (lore?: NWLoreVo) => {
    editingUlid.value = lore?.lore_ulid ?? ''
    form.lore_name = lore?.lore_name ?? ''
    form.lore_type_code = lore?.lore_type_code || 'world'
    form.content_text = lore?.content_text ?? ''
    visible.value = true
  }

  const submit = async () => {
    if (!form.lore_name.trim()) return
    saving.value = true
    try {
      const savedLore = await saveLore({
        project_ulid: props.projectUlid,
        lore_ulid: editingUlid.value || undefined,
        lore_name: form.lore_name.trim(),
        lore_type_code: form.lore_type_code,
        content_text: form.content_text,
        content_mode_code: 'markdown'
      })
      visible.value = false
      emit('saved', savedLore)
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="480px" destroy-on-close>
    <ElForm label-position="top">
      <ElFormItem label="名称">
        <ElInput
          v-model="form.lore_name"
          maxlength="128"
          show-word-limit
          placeholder="例如：灵潮海"
          @keyup.enter="submit"
        />
      </ElFormItem>

      <ElFormItem label="类型">
        <ElSelect v-model="form.lore_type_code" class="lore-form__select">
          <ElOption
            v-for="item in LORE_TYPE_OPTIONS"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
      </ElFormItem>

      <ElFormItem label="内容">
        <ElInput
          v-model="form.content_text"
          type="textarea"
          :rows="16"
          maxlength="20000"
          placeholder="支持 Markdown：# 标题、- 列表、**强调**"
        />
      </ElFormItem>

      <ElAlert type="info" :closable="false" show-icon>
        设定以 Markdown 保存，AI 起草时会作为背景资料检索。
      </ElAlert>
    </ElForm>

    <template #footer>
      <div class="lore-form__footer">
        <ElButton @click="visible = false">取消</ElButton>
        <ElButton
          type="primary"
          :loading="saving"
          :disabled="!form.lore_name.trim()"
          @click="submit"
        >
          保存
        </ElButton>
      </div>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .lore-form {
    &__select {
      width: 100%;
    }

    &__footer {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }
  }
</style>
