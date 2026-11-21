<!-- 大纲节点弹窗：新增与编辑树节点。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElOption, ElSelect } from 'element-plus'
  import { saveOutlineNode } from '@/api/outline'
  import type {
    NWOutlineNodeSaveReqVo,
    NWOutlineNodeStatusCode,
    NWOutlineNodeTypeCode,
    NWOutlineNodeVo
  } from '@/api/outline'
  import { NODE_STATUS_OPTIONS, NODE_TYPE_OPTIONS } from './meta'

  const props = defineProps<{ projectUlid: string }>()

  const emit = defineEmits<{ saved: [node: NWOutlineNodeVo] }>()

  const visible = ref(false)
  const saving = ref(false)
  const editingUlid = ref('')
  const parentUlid = ref('')
  const parentTitle = ref('')

  const form = reactive<{
    node_type_code: NWOutlineNodeTypeCode
    title_text: string
    summary_text: string
    node_status_code: NWOutlineNodeStatusCode
  }>({
    node_type_code: 'volume',
    title_text: '',
    summary_text: '',
    node_status_code: 'draft'
  })

  const title = computed(() => (editingUlid.value ? '编辑大纲节点' : '新建大纲节点'))

  /** 打开弹窗：传 node 为编辑，不传为新建；parent 决定归属层级 */
  const open = (node?: NWOutlineNodeVo, parent?: NWOutlineNodeVo) => {
    editingUlid.value = node?.outline_ulid ?? ''
    parentUlid.value = node?.parent_outline_ulid ?? parent?.outline_ulid ?? ''
    parentTitle.value = node ? '' : (parent?.title_text ?? '')
    form.node_type_code = node?.node_type_code ?? (parent ? 'chapter' : 'volume')
    form.title_text = node?.title_text ?? ''
    form.summary_text = node?.summary_text ?? ''
    form.node_status_code = node?.node_status_code ?? 'draft'
    visible.value = true
  }

  const submit = async () => {
    if (!form.title_text.trim()) return
    saving.value = true
    try {
      const payload: NWOutlineNodeSaveReqVo = {
        project_ulid: props.projectUlid,
        parent_outline_ulid: parentUlid.value,
        node_type_code: form.node_type_code,
        title_text: form.title_text.trim(),
        summary_text: form.summary_text,
        node_status_code: form.node_status_code
      }
      if (editingUlid.value) payload.outline_ulid = editingUlid.value
      const savedNode = await saveOutlineNode(payload)
      visible.value = false
      emit('saved', savedNode)
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="520px" append-to-body destroy-on-close>
    <ElForm label-position="top">
      <p v-if="parentTitle" class="node-form__parent">上级节点：{{ parentTitle }}</p>

      <div class="node-form__row">
        <ElFormItem label="节点类型">
          <ElSelect v-model="form.node_type_code" class="node-form__select">
            <ElOption
              v-for="item in NODE_TYPE_OPTIONS"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="状态">
          <ElSelect v-model="form.node_status_code" class="node-form__select">
            <ElOption
              v-for="item in NODE_STATUS_OPTIONS"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
      </div>

      <ElFormItem label="标题">
        <ElInput
          v-model="form.title_text"
          maxlength="128"
          show-word-limit
          placeholder="例如：第一卷 长夜将尽"
          @keyup.enter="submit"
        />
      </ElFormItem>

      <ElFormItem label="概要">
        <ElInput
          v-model="form.summary_text"
          type="textarea"
          :rows="4"
          maxlength="4096"
          placeholder="这一段剧情要发生什么，留给场景卡细化"
        />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" :disabled="!form.title_text.trim()" @click="submit">
        保存
      </ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .node-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }

    &__parent {
      margin-bottom: 12px;
      font-size: 12px;
      color: var(--nw-text-secondary);
    }
  }
</style>
