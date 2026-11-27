<!-- 审计报文查看器：JSON 高亮、关键字定位、复制原文与「复制为 cURL」。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElButton, ElInput, ElMessage } from 'element-plus'

  const props = defineProps<{
    /** 段落标题（请求体 / 响应体） */
    titleText: string
    /** 落库原文（可能为空：未采样） */
    rawText: string
    /** 未采样时的说明 */
    emptyText: string
    /** 复制为 cURL 的命令文本（仅请求体传入） */
    curlText?: string
  }>()

  const keyword = ref('')
  const wrapFlag = ref(false)

  /** 原文是否可解析为 JSON（决定高亮与换行方式） */
  const isJson = computed(() => {
    if (!props.rawText) return false
    try {
      JSON.parse(props.rawText)
      return true
    } catch {
      return false
    }
  })

  /** 美化后的文本（非 JSON 原样展示） */
  const prettyText = computed(() => {
    if (!props.rawText) return ''
    if (!isJson.value) return props.rawText
    try {
      return JSON.stringify(JSON.parse(props.rawText), null, 2)
    } catch {
      return props.rawText
    }
  })

  /** 高亮后的 HTML（先转义再着色，避免注入） */
  const htmlText = computed(() =>
    isJson.value ? highlightJson(prettyText.value, keyword.value) : markSearch(escapeHtml(prettyText.value), keyword.value)
  )

  /** 关键字命中次数（顶部提示用） */
  const hitCount = computed(() => countHits(prettyText.value, keyword.value))

  /** JSON 词法：字符串键、字符串值、数字、字面量 */
  const tokenPattern =
    /("(?:\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(?:\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g

  /** 逐 token 着色，并在 token 内标记关键字命中 */
  function highlightJson(text: string, searchText: string): string {
    let out = ''
    let cursor = 0
    for (const match of text.matchAll(tokenPattern)) {
      const start = match.index ?? 0
      const token = match[0]
      out += markSearch(escapeHtml(text.slice(cursor, start)), searchText)
      out += `<span class="tok-${tokenClass(token)}">${markSearch(escapeHtml(token), searchText)}</span>`
      cursor = start + token.length
    }
    return out + markSearch(escapeHtml(text.slice(cursor)), searchText)
  }

  /** 判断 token 的着色类别 */
  function tokenClass(token: string): string {
    if (token.startsWith('"')) {
      return token.trimEnd().endsWith(':') ? 'key' : 'str'
    }
    return /^(true|false|null)$/.test(token) ? 'lit' : 'num'
  }

  /** HTML 转义（报文内容不可信） */
  function escapeHtml(text: string): string {
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
  }

  /** 用 <mark> 包裹关键字命中（空关键字原样返回） */
  function markSearch(htmlTextValue: string, searchText: string): string {
    const needle = searchText.trim()
    if (!needle) return htmlTextValue
    const pattern = new RegExp(escapeRegExp(escapeHtml(needle)), 'gi')
    return htmlTextValue.replace(pattern, (hit) => `<mark class="audit-body__hit">${hit}</mark>`)
  }

  /** 转义正则元字符 */
  function escapeRegExp(text: string): string {
    return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  }

  /** 统计关键字命中次数（不区分大小写） */
  function countHits(text: string, searchText: string): number {
    const needle = searchText.trim().toLowerCase()
    if (!needle || !text) return 0
    return text.toLowerCase().split(needle).length - 1
  }

  /** 复制文本到剪贴板（浏览器拒绝时提示手动复制） */
  const copyText = async (text: string, successText: string) => {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      ElMessage.success(successText)
    } catch {
      ElMessage.warning('浏览器不允许写剪贴板，请手动选择复制')
    }
  }
</script>

<template>
  <section class="audit-body">
    <header class="audit-body__head">
      <h4 class="audit-body__title">{{ titleText }}</h4>
      <span v-if="keyword.trim()" class="nw-hint">命中 {{ hitCount }} 处</span>
      <div class="audit-body__spacer" />
      <ElInput
        v-model="keyword"
        size="small"
        clearable
        placeholder="在报文中查找"
        class="audit-body__search"
      />
      <ElButton size="small" text :disabled="!rawText" @click="copyText(rawText, '已复制原文')">
        复制
      </ElButton>
      <ElButton
        v-if="curlText"
        size="small"
        text
        :disabled="!rawText"
        @click="copyText(curlText, '已复制 cURL 命令')"
      >
        复制为 cURL
      </ElButton>
    </header>

    <div v-if="!rawText" class="audit-body__empty">{{ emptyText }}</div>
    <pre
      v-else
      class="audit-body__code"
      :class="{ 'is-wrap': wrapFlag }"
      @dblclick="wrapFlag = !wrapFlag"
      v-html="htmlText"
    />
    <p v-if="rawText" class="nw-hint audit-body__tip">双击报文可切换自动换行</p>
  </section>
</template>

<style lang="less" scoped>
  .audit-body {
    display: flex;
    flex-direction: column;
    gap: 8px;

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }

    &__title {
      margin: 0;
      font-size: 13px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }

    &__spacer {
      flex: 1;
    }

    &__search {
      width: 180px;
    }

    &__code {
      max-height: 360px;
      padding: 12px;
      overflow: auto;
      font-family: var(--nw-font-mono);
      font-size: 12px;
      line-height: 1.65;
      white-space: pre;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);

      &.is-wrap {
        white-space: pre-wrap;
        word-break: break-all;
      }

      :deep(.tok-key) {
        color: var(--el-color-primary);
      }

      :deep(.tok-str) {
        color: var(--nw-success);
      }

      :deep(.tok-num) {
        color: var(--nw-warning);
      }

      :deep(.tok-lit) {
        color: var(--el-color-danger);
      }

      :deep(.audit-body__hit) {
        padding: 0 2px;
        color: inherit;
        background: color-mix(in srgb, var(--nw-warning) 35%, transparent);
        border-radius: 3px;
      }
    }

    &__empty {
      padding: 12px;
      font-size: 12px;
      color: var(--el-text-color-placeholder);
      background: var(--nw-surface-subtle);
      border: 1px dashed var(--nw-border);
      border-radius: var(--nw-radius-md);
    }

    &__tip {
      margin: 0;
    }
  }
</style>
