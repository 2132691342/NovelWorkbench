<!-- 检索面板：关键词检索并展示命中片段与追踪信息。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElInput,
    ElInputNumber,
    ElOption,
    ElSelect,
    ElTag
  } from 'element-plus'
  import { searchKnowledge, type NWKnowledgeSearchVo } from '@/api/knowledge'
  import type { NWKnowledgeChunkVo } from '@/api/contracts'

  const props = defineProps<{ projectUlid: string }>()

  /** 检索模式选项（keyword 纯关键词 / hybrid 词法双通道融合） */
  const modeOptions = [
    { value: 'hybrid', label: '混合（关键词 + 短语）' },
    { value: 'keyword', label: '关键词' }
  ]

  const keywordText = ref('')
  const searchMode = ref('hybrid')
  const topKCount = ref(6)
  const loading = ref(false)
  const searched = ref(false)
  const result = ref<NWKnowledgeSearchVo | null>(null)

  const resultList = computed<NWKnowledgeChunkVo[]>(() => result.value?.result_list ?? [])
  const trace = computed(() => result.value?.trace ?? null)

  const escapeHtml = (text: string) =>
    text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')

  const escapeRegExp = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

  /** 命中的检索词（去掉过短词，避免满屏高亮） */
  const highlightTermList = computed(() =>
    (trace.value?.query_terms ?? []).filter((term) => term.trim().length > 0)
  )

  /** 切片文本高亮：先转义，再包裹 <mark>，避免 XSS */
  const highlightText = (text: string) => {
    const safeText = escapeHtml(text)
    const termList = highlightTermList.value
    if (termList.length === 0) return safeText
    const pattern = new RegExp(termList.map(escapeRegExp).join('|'), 'gi')
    return safeText.replace(pattern, (matched) => `<mark class="knowledge-hit">${matched}</mark>`)
  }

  const submitSearch = async () => {
    const keyword = keywordText.value.trim()
    if (!keyword) return
    if (!props.projectUlid) return
    loading.value = true
    try {
      result.value = await searchKnowledge({
        project_ulid: props.projectUlid,
        keyword_text: keyword,
        search_mode: searchMode.value,
        top_k_count: topKCount.value
      })
      searched.value = true
    } catch {
      result.value = null
      searched.value = true
    } finally {
      loading.value = false
    }
  }

  const resetSearch = () => {
    keywordText.value = ''
    result.value = null
    searched.value = false
  }
</script>

<template>
  <ElCard shadow="never" class="knowledge-search">
    <template #header>
      <div class="knowledge-search__head">
        <span class="knowledge-search__title">
          <Icon icon="mdi:database-outline" :size="15" />
          <span class="knowledge-search__title-text">知识库检索</span>
        </span>
        <span class="nw-hint">词法检索（关键词 / 混合），不依赖向量模型</span>
      </div>
    </template>

    <div class="knowledge-search__form">
      <ElInput
        v-model="keywordText"
        placeholder="输入关键词或问句，回车检索"
        clearable
        class="knowledge-search__input"
        @keyup.enter="submitSearch"
        @clear="resetSearch"
      />
      <ElSelect v-model="searchMode" class="knowledge-search__mode">
        <ElOption
          v-for="item in modeOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElInputNumber v-model="topKCount" :min="1" :max="20" :step="1" class="knowledge-search__topk" />
      <ElButton type="primary" plain size="small" :loading="loading" @click="submitSearch">
        <Icon icon="mdi:pulse" :size="14" />
        <span class="ml-4px">检索</span>
      </ElButton>
    </div>

    <div v-if="trace" class="knowledge-search__trace">
      <ElTag size="small" type="info">通道 {{ trace.source_mode || '—' }}</ElTag>
      <ElTag size="small" type="info">候选 {{ trace.candidate_count }}</ElTag>
      <ElTag size="small" type="info">入选 {{ trace.selected_ids.length }}</ElTag>
      <span v-if="trace.query_expr" class="knowledge-search__expr">表达式：{{ trace.query_expr }}</span>
    </div>

    <div v-loading="loading" class="knowledge-search__results">
      <div v-for="chunk in resultList" :key="chunk.chunk_ulid" class="knowledge-search__item">
        <div class="knowledge-search__item-head">
          <span class="knowledge-search__doc">{{ chunk.doc_name }}</span>
          <span class="nw-badge nw-badge--neutral">#{{ chunk.chunk_index + 1 }}</span>
          <span class="knowledge-search__score">{{ chunk.score_value.toFixed(3) }}</span>
          <span class="knowledge-search__source">{{ chunk.source_mode }}</span>
        </div>
        <p class="knowledge-search__text" v-html="highlightText(chunk.chunk_text)"></p>
      </div>

      <ElEmpty
        v-if="searched && !loading && resultList.length === 0"
        description="没有命中切片，换个关键词试试"
        :image-size="70"
      />
      <p v-if="!searched && !loading" class="nw-hint">
        输入关键词检索项目知识库，命中片段会标注相关度得分。
      </p>
    </div>
  </ElCard>
</template>

<style lang="less" scoped>
  .knowledge-search {
    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
    }

    &__title {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      font-size: 14px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__title-text {
      line-height: 1;
    }

    &__form {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
    }

    &__input {
      width: 320px;
    }

    &__mode {
      width: 180px;
    }

    &__topk {
      width: 96px;
    }

    &__trace {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      margin-top: 12px;
    }

    &__expr {
      font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
      font-size: 11.5px;
      color: var(--nw-text-muted);
      word-break: break-all;
    }

    &__results {
      display: flex;
      flex-direction: column;
      gap: 10px;
      min-height: 60px;
      margin-top: 12px;
    }

    &__item {
      padding: 10px 12px;
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
      background: var(--nw-surface-subtle);
    }

    &__item-head {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      margin-bottom: 6px;
    }

    &__doc {
      font-size: 12.5px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__score {
      font-size: 11.5px;
      font-variant-numeric: tabular-nums;
      color: var(--el-color-primary);
    }

    &__source {
      font-size: 11px;
      color: var(--nw-text-muted);
    }

    &__text {
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--el-text-color-regular);
      white-space: pre-wrap;
      word-break: break-word;
    }

    :deep(.knowledge-hit) {
      padding: 0 2px;
      border-radius: var(--nw-radius-sm);
      background: var(--nw-primary-light);
      color: var(--nw-ink);
    }
  }
</style>
