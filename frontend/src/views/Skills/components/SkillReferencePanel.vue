<!-- 参考资料面板：维护技能的资料条目与命中关键词。 -->
<script setup lang="ts">
  import { ref, watch } from 'vue'
  import {
    ElButton,
    ElDialog,
    ElEmpty,
    ElForm,
    ElFormItem,
    ElInput,
    ElInputNumber,
    ElMessage,
    ElMessageBox,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import {
    deleteSkillReference,
    listSkillReferences,
    saveSkillReference,
    type NWSkillReferenceSaveReqVo
  } from '@/api/skill'
  import type { NWSkillReferenceVo } from '@/api/contracts'

  const props = defineProps<{ skillUlid: string }>()

  const loading = ref(false)
  const saving = ref(false)
  const visible = ref(false)
  const list = ref<NWSkillReferenceVo[]>([])
  const editing = ref<NWSkillReferenceVo | null>(null)
  const form = ref<Omit<NWSkillReferenceSaveReqVo, 'skill_ulid'>>({
    ref_name: '',
    ref_text: '',
    keyword_list: '',
    sort_order: 0
  })

  const load = async () => {
    if (!props.skillUlid) {
      list.value = []
      return
    }
    loading.value = true
    try {
      list.value = await listSkillReferences(props.skillUlid)
    } catch {
      list.value = []
    } finally {
      loading.value = false
    }
  }

  const open = (row?: NWSkillReferenceVo) => {
    editing.value = row ?? null
    form.value = {
      ref_ulid: row?.ref_ulid,
      ref_name: row?.ref_name ?? '',
      ref_text: row?.ref_text ?? '',
      keyword_list: row?.keyword_list ?? '',
      sort_order: row?.sort_order ?? 0
    }
    visible.value = true
  }

  const submit = async () => {
    if (!form.value.ref_name.trim()) {
      ElMessage.warning('请填写资料名')
      return
    }
    if (!form.value.ref_text.trim()) {
      ElMessage.warning('请填写资料正文')
      return
    }
    saving.value = true
    try {
      await saveSkillReference({
        skill_ulid: props.skillUlid,
        ref_ulid: form.value.ref_ulid,
        ref_name: form.value.ref_name,
        ref_text: form.value.ref_text,
        keyword_list: form.value.keyword_list,
        sort_order: form.value.sort_order
      })
      ElMessage.success('参考资料已保存')
      visible.value = false
      await load()
    } finally {
      saving.value = false
    }
  }

  const remove = async (row: NWSkillReferenceVo) => {
    try {
      await ElMessageBox.confirm(`将删除参考资料「${row.ref_name}」，确定吗？`, '删除资料', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteSkillReference(row.ref_ulid)
    ElMessage.success('参考资料已删除')
    await load()
  }

  watch(
    () => props.skillUlid,
    (value) => {
      if (value) void load()
      else list.value = []
    },
    { immediate: true }
  )
</script>

<template>
  <div class="ref-panel">
    <div class="ref-panel__head">
      <span class="ref-panel__title">参考资料</span>
      <span class="nw-hint">命中关键词时按排序注入上下文，留空表示总是注入</span>
      <div class="ref-panel__spacer" />
      <ElButton size="small" plain @click="open()">
        <Icon icon="mdi:plus" :size="13" />
        <span class="ml-4px">新增资料</span>
      </ElButton>
    </div>

    <ElTable :data="list" v-loading="loading" size="small" max-height="240">
      <ElTableColumn prop="ref_name" label="资料名" min-width="120" show-overflow-tooltip />
      <ElTableColumn prop="keyword_list" label="关键词" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.keyword_list">{{ row.keyword_list }}</span>
          <span v-else class="nw-hint">总是注入</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="sort_order" label="排序" width="70" />
      <ElTableColumn label="操作" width="120">
        <template #default="{ row }">
          <ElButton size="small" text @click="open(row as NWSkillReferenceVo)">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="remove(row as NWSkillReferenceVo)">删除</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="还没有参考资料" :image-size="56" />
      </template>
    </ElTable>

    <ElDialog
      v-model="visible"
      :title="editing ? '编辑参考资料' : '新增参考资料'"
      width="520px"
      append-to-body
      destroy-on-close
    >
      <ElForm :model="form" label-position="top">
        <ElFormItem label="资料名">
          <ElInput v-model="form.ref_name" maxlength="64" placeholder="例如：文风样例" />
        </ElFormItem>
        <ElFormItem label="正文">
          <ElInput v-model="form.ref_text" type="textarea" :rows="6" maxlength="4000" show-word-limit />
        </ElFormItem>
        <div class="ref-panel__row">
          <ElFormItem label="触发关键词（逗号分隔）">
            <ElInput v-model="form.keyword_list" placeholder="雨夜,追车,打斗" />
          </ElFormItem>
          <ElFormItem label="排序号">
            <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
          </ElFormItem>
        </div>
      </ElForm>
      <template #footer>
        <ElButton @click="visible = false">取消</ElButton>
        <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style lang="less" scoped>
  .ref-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding-top: 12px;
    border-top: 1px solid var(--nw-border-soft);

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
    }

    &__title {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__spacer {
      flex: 1;
    }

    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 2fr 1fr;
    }
  }
</style>
