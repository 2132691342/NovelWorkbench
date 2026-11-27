<!-- 提示词库页：按 4 个创作流程分组，每流程两张卡片，一眼看出什么时候该点哪一条。
     身份提示词不再内置，只能由作者自建；库内没有时对话走「默认（内置作者人格）」。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTag
  } from 'element-plus'
  import { deletePrompt, listPrompts, savePrompt } from '@/api/prompt'
  import type { NWPromptVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatRelativeTime } from '@/utils/time'
  import PromptEditDrawer from './components/PromptEditDrawer.vue'
  import PromptPreviewDrawer from './components/PromptPreviewDrawer.vue'

  /** 分组结构：按流程归组，无流程域的自建提示词归入「自建提示词」 */
  interface PromptGroup {
    groupKey: string
    groupName: string
    rows: NWPromptVo[]
  }

  const projectStore = useProjectStore()

  const loading = ref(false)
  const rows = ref<NWPromptVo[]>([])
  const projectUlid = ref(projectStore.currentProjectUlid)
  const enabledOnly = ref(false)
  const kindCode = ref('')
  const togglingUlid = ref('')
  const rowAction = useRowAction()

  const drawer = ref<InstanceType<typeof PromptEditDrawer>>()
  const previewVisible = ref(false)
  const previewPrompt = ref<NWPromptVo | null>(null)

  const projectNameOf = (ulid: string) =>
    projectStore.projectOptions.find((item) => item.project_ulid === ulid)?.project_name ?? ''

  /** 内置流程固定顺序：开书规划 → 写正文 → 修问题 → 质检；自建分组排最后 */
  const FLOW_ORDER = ['plan', 'draft', 'revise', 'check']

  const kindLabel = (code: string) => (code === 'system' ? '身份' : '任务')

  /** 卡片正文摘要：只给前 80 字，完整正文进预览抽屉 */
  const excerptOf = (promptText: string) => {
    const plainText = promptText.replace(/\s+/g, ' ').trim()
    return plainText.length > 80 ? `${plainText.slice(0, 80)}…` : plainText
  }

  const load = async () => {
    loading.value = true
    try {
      rows.value = await listPrompts({
        project_ulid: projectUlid.value || undefined,
        prompt_kind_code: kindCode.value || undefined,
        enabled_only: enabledOnly.value || undefined
      })
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  const groups = computed<PromptGroup[]>(() => {
    const groupMap = new Map<string, PromptGroup>()
    for (const row of rows.value) {
      const groupKey = row.domain_code || 'custom'
      const groupName = row.domain_name || '自建提示词'
      const existing = groupMap.get(groupKey)
      if (existing) {
        existing.rows.push(row)
      } else {
        groupMap.set(groupKey, { groupKey, groupName, rows: [row] })
      }
    }
    return [...groupMap.values()]
      .map((group) => ({
        ...group,
        rows: [...group.rows].sort((left, right) => {
          if (left.prompt_kind_code !== right.prompt_kind_code) {
            return left.prompt_kind_code === 'system' ? -1 : 1
          }
          return left.sort_order - right.sort_order
        })
      }))
      .sort((left, right) => {
        const leftIndex = FLOW_ORDER.indexOf(left.groupKey)
        const rightIndex = FLOW_ORDER.indexOf(right.groupKey)
        if (leftIndex === -1 && rightIndex === -1) return left.groupName.localeCompare(right.groupName, 'zh')
        if (leftIndex === -1) return 1
        if (rightIndex === -1) return -1
        return leftIndex - rightIndex
      })
  })

  const builtinCount = computed(() => rows.value.filter((row) => row.builtin_flag).length)
  const enabledCount = computed(() => rows.value.filter((row) => row.enabled_flag).length)

  /** 开关切换需回传完整提示词体（保存为整体覆盖） */
  const toggleEnabled = async (row: NWPromptVo, enabled: boolean) => {
    togglingUlid.value = row.prompt_ulid
    try {
      await savePrompt({
        prompt_ulid: row.prompt_ulid,
        prompt_code: row.prompt_code,
        prompt_name: row.prompt_name,
        prompt_kind_code: row.prompt_kind_code,
        scope_code: row.scope_code,
        project_ulid: row.project_ulid || undefined,
        category_code: row.category_code,
        description_text: row.description_text,
        prompt_text: row.prompt_text,
        enabled_flag: enabled,
        sort_order: row.sort_order
      })
      ElMessage.success(enabled ? '提示词已启用' : '提示词已停用')
      await load()
    } catch {
      row.enabled_flag = !enabled
    } finally {
      togglingUlid.value = ''
    }
  }

  const remove = async (row: NWPromptVo) => {
    try {
      await ElMessageBox.confirm(`将删除提示词「${row.prompt_name}」，确定吗？`, '删除提示词', {
        type: 'warning'
      })
    } catch {
      return
    }
    await rowAction.run(row.prompt_ulid, async () => {
      await deletePrompt(row.prompt_ulid)
      ElMessage.success('提示词已删除')
      await load()
    })
  }

  const openPreview = (row: NWPromptVo) => {
    previewPrompt.value = row
    previewVisible.value = true
  }

  watch([projectUlid, enabledOnly, kindCode], () => void load())

  onMounted(async () => {
    const initialUlid = projectUlid.value
    if (projectStore.projectOptions.length === 0) {
      await projectStore.loadProjectOptions()
      projectUlid.value = projectStore.currentProjectUlid
    }
    // 项目切换会触发 watch 里的加载，这里只处理未切换的情况
    if (projectUlid.value === initialUlid) await load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElSelect
        v-model="projectUlid"
        placeholder="仅全局提示词"
        clearable
        filterable
        class="prompts__project"
      >
        <ElOption
          v-for="item in projectStore.projectOptions"
          :key="item.project_ulid"
          :label="item.project_name"
          :value="item.project_ulid"
        />
      </ElSelect>
      <ElSelect v-model="kindCode" placeholder="全部种类" clearable class="prompts__kind">
        <ElOption label="全部种类" value="" />
        <ElOption label="任务提示词（/ 触发）" value="command" />
        <ElOption label="身份提示词（仅自建，会话常驻）" value="system" />
      </ElSelect>
      <ElButton type="primary" plain @click="drawer?.open(undefined, projectUlid)">
        <Icon icon="mdi:plus" :size="15" />
        <span class="ml-4px">新建提示词</span>
      </ElButton>
      <div class="prompts__spacer" />
      <div class="prompts__filter">
        <ElSwitch v-model="enabledOnly" size="small" />
        <span class="nw-hint">仅看启用</span>
      </div>
      <span class="nw-hint">
        共 {{ rows.length }} 条 · 启用 {{ enabledCount }} · 内置 {{ builtinCount }}
      </span>
    </div>

    <div v-loading="loading" class="prompts__groups">
      <ElCard v-for="group in groups" :key="group.groupKey" shadow="never" class="prompts__group">
        <template #header>
          <div class="prompts__group-head">
            <span class="prompts__group-title">{{ group.groupName }}</span>
            <span class="nw-hint">{{ group.rows.length }} 条</span>
          </div>
        </template>

        <div class="prompts__cards">
          <article
            v-for="row in group.rows"
            :key="row.prompt_ulid"
            class="prompt-card"
            :class="{ 'is-disabled': !row.enabled_flag }"
            @click="openPreview(row)"
          >
            <div class="prompt-card__head">
              <span class="prompt-card__title">{{ row.prompt_name }}</span>
              <code class="prompt-card__code">
                {{ row.prompt_kind_code === 'command' ? '/' : '' }}{{ row.prompt_code }}
              </code>
            </div>
            <p class="prompt-card__when">
              {{ row.description_text || '未填写使用场景' }}
            </p>
            <p class="prompt-card__excerpt">{{ excerptOf(row.prompt_text) }}</p>
            <div class="prompt-card__tags">
              <ElTag v-if="row.builtin_flag" size="small" type="info" effect="plain">内置</ElTag>
              <ElTag v-else size="small" type="success" effect="plain">自建</ElTag>
              <ElTag size="small" effect="plain">{{ kindLabel(row.prompt_kind_code) }}</ElTag>
              <ElTag v-if="row.customized_flag" size="small" type="warning" effect="plain">
                已自定义
              </ElTag>
              <ElTag v-if="row.overridden_flag" size="small" type="danger" effect="plain">
                被项目覆盖
              </ElTag>
              <span class="prompt-card__scope">
                {{ row.scope_code === 'project'
                  ? `项目 · ${projectNameOf(row.project_ulid) || '未知项目'}`
                  : '全局' }}
                · {{ formatRelativeTime(row.updated_at) }}
              </span>
            </div>
            <div class="prompt-card__actions" @click.stop>
              <ElSwitch
                :model-value="row.enabled_flag"
                size="small"
                :loading="togglingUlid === row.prompt_ulid"
                @update:model-value="toggleEnabled(row, Boolean($event))"
              />
              <span class="nw-hint">{{ row.prompt_text.length }} 字符</span>
              <div class="prompts__spacer" />
              <ElButton size="small" plain @click="openPreview(row)">预览</ElButton>
              <ElButton size="small" plain @click="drawer?.open(row, projectUlid)">编辑</ElButton>
              <ElButton
                size="small"
                plain
                type="danger"
                :loading="rowAction.isPending(row.prompt_ulid)"
                :disabled="row.builtin_flag || rowAction.busyFlag.value"
                @click="remove(row)"
              >
                删除
              </ElButton>
            </div>
          </article>
        </div>
      </ElCard>

      <ElEmpty v-if="!loading && groups.length === 0" description="还没有提示词，先建一条吧" />
    </div>

    <PromptEditDrawer ref="drawer" :projects="projectStore.projectOptions" @saved="load" />
    <PromptPreviewDrawer v-model:visible="previewVisible" :prompt="previewPrompt" />
  </div>
</template>

<style lang="less" scoped>
  .prompts {
    &__project {
      width: 180px;
    }

    &__kind {
      width: 220px;
    }

    &__spacer {
      flex: 1;
    }

    &__filter {
      display: flex;
      gap: 6px;
      align-items: center;
    }

    &__groups {
      display: flex;
      flex-direction: column;
      gap: 14px;
    }

    &__group {
      border-radius: var(--nw-radius-lg);
    }

    &__group-head {
      display: flex;
      gap: 10px;
      align-items: baseline;
    }

    &__group-title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__cards {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 12px;
    }
  }

  .prompt-card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px 14px;
    cursor: pointer;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border-soft);
    border-radius: var(--nw-radius-md);
    transition:
      border-color 0.18s ease,
      box-shadow 0.18s ease,
      transform 0.18s ease;

    &:hover {
      border-color: var(--nw-primary);
      box-shadow: 0 6px 18px rgb(15 23 42 / 8%);
      transform: translateY(-1px);
    }

    &.is-disabled {
      opacity: 0.62;
    }

    &__head {
      display: flex;
      gap: 8px;
      align-items: baseline;
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__code {
      padding: 1px 6px;
      font-family: var(--nw-font-mono);
      font-size: 11px;
      color: var(--nw-primary);
      background: var(--nw-primary-soft);
      border-radius: var(--nw-radius-sm);
    }

    &__when {
      margin: 0;
      font-size: 12.5px;
      line-height: 1.6;
      color: var(--nw-text-secondary);
    }

    &__excerpt {
      margin: 0;
      font-size: 11.5px;
      line-height: 1.7;
      color: var(--nw-text-muted);
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      align-items: center;
    }

    &__scope {
      font-size: 11px;
      color: var(--nw-text-muted);
    }

    &__actions {
      display: flex;
      gap: 6px;
      align-items: center;
      padding-top: 8px;
      border-top: 1px dashed var(--nw-border-soft);
    }
  }
</style>
