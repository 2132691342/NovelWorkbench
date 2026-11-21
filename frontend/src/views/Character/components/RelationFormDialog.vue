<!-- 关系编辑弹窗：维护关系的起点、终点与描述。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDialog,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    ElOption,
    ElSelect
  } from 'element-plus'
  import { saveCharacterRelation } from '@/api/character'
  import type { NWCharacterNodeVo } from '@/api/character'

  const props = defineProps<{ projectUlid: string; nodeList: NWCharacterNodeVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  /** 关系类型选项（后端为自由文本，这里给出常用取值） */
  const relationTypeOptions = [
    { value: 'friend', label: '朋友' },
    { value: 'enemy', label: '敌对' },
    { value: 'lover', label: '恋人' },
    { value: 'kin', label: '亲属' },
    { value: 'teacher', label: '师徒' },
    { value: 'rival', label: '对手' },
    { value: 'subordinate', label: '上下级' }
  ]

  const visible = ref(false)
  const saving = ref(false)
  const form = reactive({
    from_character_ulid: '',
    to_character_ulid: '',
    relation_type_code: 'friend',
    relation_note: ''
  })

  const canSubmit = computed(
    () =>
      form.from_character_ulid.length > 0 &&
      form.to_character_ulid.length > 0 &&
      form.from_character_ulid !== form.to_character_ulid
  )

  const open = () => {
    form.from_character_ulid = props.nodeList[0]?.character_ulid ?? ''
    form.to_character_ulid = props.nodeList[1]?.character_ulid ?? ''
    form.relation_type_code = 'friend'
    form.relation_note = ''
    visible.value = true
  }

  const submit = async () => {
    if (!canSubmit.value) {
      ElMessage.warning('请选择两个不同的角色')
      return
    }
    saving.value = true
    try {
      await saveCharacterRelation({
        project_ulid: props.projectUlid,
        from_character_ulid: form.from_character_ulid,
        to_character_ulid: form.to_character_ulid,
        relation_type_code: form.relation_type_code,
        relation_note: form.relation_note.trim()
      })
      ElMessage.success('关系已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" title="新建关系" width="460px" append-to-body>
    <ElForm :model="form" label-position="top">
      <ElFormItem label="起点角色" required>
        <ElSelect v-model="form.from_character_ulid" class="relation-form__select" filterable>
          <ElOption
            v-for="node in nodeList"
            :key="node.character_ulid"
            :label="node.character_name"
            :value="node.character_ulid"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="终点角色" required>
        <ElSelect v-model="form.to_character_ulid" class="relation-form__select" filterable>
          <ElOption
            v-for="node in nodeList"
            :key="node.character_ulid"
            :label="node.character_name"
            :value="node.character_ulid"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="关系类型" required>
        <ElSelect v-model="form.relation_type_code" class="relation-form__select">
          <ElOption
            v-for="item in relationTypeOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="关系说明">
        <ElInput
          v-model="form.relation_note"
          type="textarea"
          :rows="2"
          maxlength="256"
          show-word-limit
          placeholder="一句话说明这段关系（可选）"
        />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton size="small" @click="visible = false">取消</ElButton>
      <ElButton
        type="primary"
        size="small"
        :loading="saving"
        :disabled="!canSubmit"
        @click="submit"
      >
        创建
      </ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .relation-form {
    &__select {
      width: 100%;
    }
  }
</style>
