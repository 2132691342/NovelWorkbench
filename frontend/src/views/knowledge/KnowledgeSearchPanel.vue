<!--
  知识库检索区：关键词 / 词法混合模式 + top_k_count，命中卡片展示来源与高亮文本。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { searchKnowledge, type NWKnowledgeSearchReqVo } from '@/api/knowledge'
import type { NWKnowledgeChunkVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'

const props = defineProps<{
  projectUlid: string
}>()

const { t } = useI18n()

/** 检索模式：keyword 关键词 / hybrid 词法混合 */
type NWSearchMode = 'keyword' | 'hybrid'

const searchMode = ref<NWSearchMode>('hybrid')
const keywordText = ref('')
const queryText = ref('')
const topKCount = ref(6)
const searchingFlag = ref(false)
const searchedFlag = ref(false)
const resultList = ref<NWKnowledgeChunkVo[]>([])
const traceText = ref('')

/** 文本高亮片段 */
interface NWHighlightSegment {
  text: string
  hit_flag: boolean
}

/** 检索模式展示文案 */
function searchModeText(mode: string): string {
  switch (mode) {
    case 'keyword':
      return t('knowledge.search.modeKeyword')
    default:
      return t('knowledge.search.modeHybrid')
  }
}

/** source_mode 徽标文案 */
function sourceModeText(sourceMode: string): string {
  switch (sourceMode) {
    case 'keyword':
      return t('knowledge.search.sourceKeyword')
    default:
      return t('knowledge.search.sourceHybrid')
  }
}

/** 拆分命中高亮片段（不命中区间原样返回，避免 v-html 注入） */
function buildHighlightSegments(text: string): NWHighlightSegment[] {
  const terms = [keywordText.value, queryText.value]
    .flatMap((raw) => raw.split(/\s+/))
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
  if (terms.length === 0) {
    return [{ text, hit_flag: false }]
  }
  const pattern = new RegExp(terms.map((item) => item.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|'), 'gi')
  const segments: NWHighlightSegment[] = []
  let lastIndex = 0
  for (const match of text.matchAll(pattern)) {
    const matchIndex = match.index ?? 0
    if (matchIndex > lastIndex) {
      segments.push({ text: text.slice(lastIndex, matchIndex), hit_flag: false })
    }
    segments.push({ text: match[0], hit_flag: true })
    lastIndex = matchIndex + match[0].length
  }
  if (lastIndex < text.length) {
    segments.push({ text: text.slice(lastIndex), hit_flag: false })
  }
  return segments
}
/** 检索追踪摘要：让召回依据可解释 */
function buildTraceText(trace: {
  query_terms: string[]
  candidate_count: number
  reranked: boolean
  rerank_note: string
  source_mode: string
}): string {
  const terms = trace.query_terms.slice(0, 8).join(' / ') || '—'
  return `查询词 ${terms} · 候选 ${trace.candidate_count} · 来源 ${trace.source_mode} · 重排 ${trace.reranked ? '是' : '否'}${trace.rerank_note ? `（${trace.rerank_note}）` : ''}`
}

/** 执行检索 */
async function handleSearch(): Promise<void> {
  if (!props.projectUlid) {
    return
  }
  const req: NWKnowledgeSearchReqVo = {
    project_ulid: props.projectUlid,
    search_mode: searchMode.value,
    top_k_count: topKCount.value,
  }
  req.keyword_text = keywordText.value || undefined
  req.query_text = queryText.value || undefined
  if (!req.keyword_text && !req.query_text) {
    return
  }
  searchingFlag.value = true
  try {
    const searchVo = await searchKnowledge(req)
    resultList.value = Array.isArray(searchVo.result_list) ? searchVo.result_list : []
    searchedFlag.value = true
    traceText.value = buildTraceText(searchVo.trace)
  } catch {
    // 失败提示由 http 层统一弹出
    resultList.value = []
    searchedFlag.value = true
  } finally {
    searchingFlag.value = false
  }
}
</script>

<template>
  <div class="knowledge-search">
    <div class="knowledge-search__bar">
      <el-radio-group v-model="searchMode" size="default">
        <el-radio-button value="keyword">{{ searchModeText('keyword') }}</el-radio-button>
        <el-radio-button value="hybrid">{{ searchModeText('hybrid') }}</el-radio-button>
      </el-radio-group>
      <el-input
        v-model="keywordText"
        class="knowledge-search__input"
        clearable
        :placeholder="t('knowledge.search.keywordPlaceholder')"
        @keyup.enter="handleSearch"
      />
      <el-input
        v-if="searchMode !== 'keyword'"
        v-model="queryText"
        class="knowledge-search__input"
        clearable
        :placeholder="t('knowledge.search.queryPlaceholder')"
        @keyup.enter="handleSearch"
      />
      <span class="knowledge-search__topk">{{ t('knowledge.search.topK') }}</span>
      <el-input-number v-model="topKCount" :min="1" :max="20" size="default" />
      <NWButton size="small" type="primary" :loading="searchingFlag" @click="handleSearch">
        {{ t('knowledge.search.run') }}
      </NWButton>
    </div>
    <p v-if="traceText" class="knowledge-search__trace">{{ traceText }}</p>
    <NWEmpty
      v-else-if="searchedFlag && resultList.length === 0"
      :description_text="t('knowledge.search.resultEmpty')"
    />
    <div v-else class="knowledge-search__results">
      <article
        v-for="chunkVo in resultList"
        :key="chunkVo.chunk_ulid"
        class="knowledge-search__card"
      >
        <div class="knowledge-search__card-head">
          <span class="knowledge-search__doc-name" :title="chunkVo.doc_name">
            {{ chunkVo.doc_name }}
          </span>
          <span class="knowledge-search__meta">
            #{{ chunkVo.chunk_index }} · {{ t('knowledge.search.score') }}
            {{ chunkVo.score_value.toFixed(3) }}
          </span>
          <span class="knowledge-search__source" :class="`is-${chunkVo.source_mode}`">
            {{ sourceModeText(chunkVo.source_mode) }}
          </span>
        </div>
        <p class="knowledge-search__text">
          <template
            v-for="(segment, segmentIndex) in buildHighlightSegments(chunkVo.chunk_text)"
            :key="segmentIndex"
          >
            <mark v-if="segment.hit_flag" class="knowledge-search__hit">{{ segment.text }}</mark>
            <template v-else>{{ segment.text }}</template>
          </template>
        </p>
      </article>
    </div>
  </div>
</template>

<style scoped>
.knowledge-search {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.knowledge-search__bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.knowledge-search__input {
  width: 220px;
}

.knowledge-search__topk {
  font-size: 13px;
  color: var(--text-secondary);
}

.knowledge-search__results {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.knowledge-search__card {
  padding: 14px 16px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  box-shadow: var(--shadow-card);
}

.knowledge-search__card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.knowledge-search__doc-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 40%;
}

.knowledge-search__meta {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.knowledge-search__trace {
  margin: 0;
  padding: 8px 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-secondary);
  background: var(--primary-light);
  border-radius: var(--radius-base);
}
.knowledge-search__source {
  margin-left: auto;
  flex-shrink: 0;
  padding: 2px 10px;
  font-size: 12px;
  border-radius: var(--radius-pill);
  color: var(--info);
  background-color: var(--info-border);
}

.knowledge-search__source.is-keyword {
  color: var(--warning);
  background-color: var(--warning-border);
}


.knowledge-search__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.8;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.knowledge-search__hit {
  padding: 0 2px;
  color: var(--text-main);
  background-color: #fde68a;
  border-radius: 3px;
}
</style>
