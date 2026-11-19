<!--
  知识库检索测试：关键词 / 词法混合两条通道，展示可解释的召回过程与命中切片。
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { searchKnowledge, type NWKnowledgeSearchReqVo } from '@/api/knowledge'
import type { NWKnowledgeChunkVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  projectUlid: string
}>()

const { t } = useI18n()

type NWSearchMode = 'keyword' | 'hybrid'

const searchMode = ref<NWSearchMode>('hybrid')
const keywordText = ref('')
const topKCount = ref(6)
const searchingFlag = ref(false)
const searchedFlag = ref(false)
const resultList = ref<NWKnowledgeChunkVo[]>([])
const traceVo = ref<{
  query_terms: string[]
  query_expr: string
  candidate_count: number
  reranked: boolean
  rerank_note: string
  selected_ids: string[]
  source_mode: string
} | null>(null)

interface NWHighlightSegment {
  text: string
  hit_flag: boolean
}

const modeHintText = computed(() =>
  searchMode.value === 'keyword'
    ? t('knowledge.search.keywordHint')
    : t('knowledge.search.hybridHint'),
)

/** 拆分命中高亮片段（不命中区间原样返回，避免 v-html 注入） */
function buildHighlightSegments(text: string): NWHighlightSegment[] {
  const terms = (traceVo.value?.query_terms ?? [])
    .flatMap((raw) => raw.split(/\s+/))
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
  if (terms.length === 0) {
    return [{ text, hit_flag: false }]
  }
  const pattern = new RegExp(
    terms.map((item) => item.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|'),
    'gi',
  )
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

function sourceModeText(sourceMode: string): string {
  switch (sourceMode) {
    case 'keyword':
      return t('knowledge.search.sourceKeyword')
    case 'phrase':
      return t('knowledge.search.sourcePhrase')
    case 'rerank':
      return t('knowledge.search.sourceRerank')
    default:
      return t('knowledge.search.sourceHybrid')
  }
}

/** 执行检索 */
async function handleSearch(): Promise<void> {
  if (!props.projectUlid || !keywordText.value.trim()) {
    return
  }
  const req: NWKnowledgeSearchReqVo = {
    project_ulid: props.projectUlid,
    keyword_text: keywordText.value.trim(),
    search_mode: searchMode.value,
    top_k_count: topKCount.value,
  }
  searchingFlag.value = true
  try {
    const searchVo = await searchKnowledge(req)
    resultList.value = Array.isArray(searchVo.result_list) ? searchVo.result_list : []
    traceVo.value = searchVo.trace
    searchedFlag.value = true
  } catch {
    resultList.value = []
    traceVo.value = null
    searchedFlag.value = true
  } finally {
    searchingFlag.value = false
  }
}

/** 清空结果 */
function handleReset(): void {
  keywordText.value = ''
  resultList.value = []
  traceVo.value = null
  searchedFlag.value = false
}
</script>

<template>
  <div class="knowledge-search">
    <!-- 检索入口 -->
    <section class="knowledge-search__hero">
      <div class="knowledge-search__hero-head">
        <span class="knowledge-search__hero-icon"><NWIcon icon_code="search" :size_px="19" /></span>
        <div>
          <h3 class="knowledge-search__hero-title">{{ t('knowledge.search.title') }}</h3>
          <p class="knowledge-search__hero-desc">{{ modeHintText }}</p>
        </div>
      </div>

      <div class="knowledge-search__bar">
        <el-input
          v-model="keywordText"
          class="knowledge-search__input"
          size="large"
          clearable
          :placeholder="t('knowledge.search.placeholder')"
          @keyup.enter="handleSearch"
        >
          <template #prefix><NWIcon icon_code="search" :size_px="16" /></template>
        </el-input>
        <NWButton type="primary" size="large" :loading="searchingFlag" @click="handleSearch">
          {{ t('knowledge.search.run') }}
        </NWButton>
      </div>

      <div class="knowledge-search__options">
        <div class="knowledge-search__mode-group">
          <button
            type="button"
            class="knowledge-search__mode"
            :class="{ 'is-active': searchMode === 'hybrid' }"
            @click="searchMode = 'hybrid'"
          >
            {{ t('knowledge.search.modeHybrid') }}
          </button>
          <button
            type="button"
            class="knowledge-search__mode"
            :class="{ 'is-active': searchMode === 'keyword' }"
            @click="searchMode = 'keyword'"
          >
            {{ t('knowledge.search.modeKeyword') }}
          </button>
        </div>
        <label class="knowledge-search__topk">
          <span>{{ t('knowledge.search.topK') }}</span>
          <el-input-number v-model="topKCount" :min="1" :max="20" size="small" controls-position="right" />
        </label>
        <button v-if="searchedFlag" type="button" class="knowledge-search__reset" @click="handleReset">
          {{ t('knowledge.search.clear') }}
        </button>
      </div>
    </section>

    <!-- 检索追踪 -->
    <section v-if="traceVo" class="knowledge-search__trace">
      <div class="knowledge-search__trace-head">
        <NWIcon icon_code="info" :size_px="14" />
        <strong>{{ t('knowledge.search.traceLabel') }}</strong>
        <span class="knowledge-search__trace-source">{{ sourceModeText(traceVo.source_mode) }}</span>
      </div>
      <div class="knowledge-search__trace-grid">
        <div>
          <span>{{ t('knowledge.search.terms') }}</span>
          <p>{{ traceVo.query_terms.join(' / ') || '—' }}</p>
        </div>
        <div>
          <span>{{ t('knowledge.search.candidates') }}</span>
          <p>{{ traceVo.candidate_count }}</p>
        </div>
        <div>
          <span>{{ t('knowledge.search.rerank') }}</span>
          <p>{{ traceVo.reranked ? t('knowledge.search.rerankYes') : t('knowledge.search.rerankNo') }}</p>
        </div>
      </div>
      <p v-if="traceVo.rerank_note" class="knowledge-search__trace-note">
        {{ traceVo.rerank_note }}
      </p>
    </section>

    <!-- 结果 -->
    <div v-if="searchingFlag" class="knowledge-search__loading">
      <div v-for="index in 3" :key="index" class="knowledge-search__skeleton" />
    </div>
    <NWEmpty
      v-else-if="searchedFlag && resultList.length === 0"
      icon_code="search"
      :title_text="t('knowledge.search.resultEmpty')"
      :description_text="t('knowledge.search.emptyDesc')"
    />
    <div v-else-if="resultList.length > 0" class="knowledge-search__result-list">
      <div class="knowledge-search__result-head">
        <span>{{ t('knowledge.search.resultCount', { count: resultList.length }) }}</span>
      </div>
      <article v-for="chunkVo in resultList" :key="chunkVo.chunk_ulid" class="knowledge-search__card">
        <div class="knowledge-search__card-head">
          <span class="knowledge-search__doc-name" :title="chunkVo.doc_name">{{ chunkVo.doc_name }}</span>
          <span class="knowledge-search__meta">#{{ chunkVo.chunk_index + 1 }}</span>
          <span class="knowledge-search__source" :class="`is-${chunkVo.source_mode}`">
            {{ sourceModeText(chunkVo.source_mode) }}
          </span>
          <span class="knowledge-search__score">{{ chunkVo.score_value.toFixed(3) }}</span>
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

    <!-- 初始引导 -->
    <div v-else class="knowledge-search__initial">
      <div class="knowledge-search__initial-icon"><NWIcon icon_code="database" :size_px="24" /></div>
      <h4>{{ t('knowledge.search.initialTitle') }}</h4>
      <p>{{ t('knowledge.search.initialDesc') }}</p>
      <div class="knowledge-search__samples">
        <button
          v-for="sampleText in [t('knowledge.search.sampleOne'), t('knowledge.search.sampleTwo'), t('knowledge.search.sampleThree')]"
          :key="sampleText"
          type="button"
          class="knowledge-search__sample"
          @click="keywordText = sampleText; handleSearch()"
        >
          {{ sampleText }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped src="./KnowledgeSearchPanel.css"></style>
