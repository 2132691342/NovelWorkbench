<!-- 世界观设定页：按分类展示与检索设定条目。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect
  } from 'element-plus'
  import { deleteLore, listLore } from '@/api/lore'
  import type { NWLoreVo } from '@/api/lore'
  import { useProjectStore } from '@/store/modules/project'
  import { formatRelativeTime } from '@/utils/time'
  import LoreDrawer from './components/LoreDrawer.vue'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  const loreList = ref<NWLoreVo[]>([])
  const loading = ref(false)
  const keywordText = ref('')
  const typeCode = ref('')

  const drawer = ref<InstanceType<typeof LoreDrawer> | null>(null)

  /** 类型筛选与卡片徽标文案 */
  const typeOptions = [
    { value: '', label: '全部类型' },
    { value: 'world', label: '世界规则' },
    { value: 'rule', label: '体系法则' },
    { value: 'item', label: '物品道具' },
    { value: 'place', label: '地点场景' },
    { value: 'event', label: '历史事件' }
  ]

  const typeLabel = (code: string) =>
    typeOptions.find((item) => item.value === code)?.label ?? (code || '未分类')

  /** 卡片正文摘要：去掉 Markdown 记号，截断到 90 字 */
  const contentSnippet = (lore: NWLoreVo): string => {
    const plainText = lore.content_text
      .replace(/[#>*`_~-]/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
    return plainText.length > 90 ? `${plainText.slice(0, 90)}…` : plainText || '还没有内容'
  }

  /** 加载设定列表（类型 + 关键词由后端过滤） */
  const loadLoreList = async () => {
    if (!projectUlid.value) {
      loreList.value = []
      return
    }
    loading.value = true
    try {
      loreList.value = await listLore({
        project_ulid: projectUlid.value,
        lore_type_code: typeCode.value || undefined,
        keyword_text: keywordText.value.trim() || undefined
      })
    } catch {
      loreList.value = []
    } finally {
      loading.value = false
    }
  }

  const resetFilters = () => {
    keywordText.value = ''
    typeCode.value = ''
    void loadLoreList()
  }

  const handleCreate = () => {
    if (!projectUlid.value) {
      ElMessage.warning('请先在顶栏选择项目')
      return
    }
    drawer.value?.open()
  }

  const handleEdit = (lore: NWLoreVo) => drawer.value?.open(lore)

  const handleDelete = async (lore: NWLoreVo) => {
    try {
      await ElMessageBox.confirm(`将删除设定「${lore.lore_name}」，删除后不可恢复。`, '删除设定', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteLore(lore.lore_ulid)
    ElMessage.success('已删除设定')
    await loadLoreList()
  }

  const handleSaved = async () => {
    ElMessage.success('设定已保存')
    await loadLoreList()
  }

  watch(projectUlid, async (nextUlid) => {
    if (!nextUlid) {
      loreList.value = []
      return
    }
    await loadLoreList()
  })

  onMounted(async () => {
    if (!projectStore.projectOptions.length) await projectStore.loadProjectOptions()
    if (projectUlid.value) await loadLoreList()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="keywordText"
        class="lore__search"
        placeholder="搜索设定名或内容"
        clearable
        @keyup.enter="loadLoreList()"
        @clear="loadLoreList()"
      />
      <ElSelect v-model="typeCode" class="lore__type" placeholder="全部类型" @change="loadLoreList()">
        <ElOption
          v-for="item in typeOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton size="small" plain @click="loadLoreList()">
        <Icon icon="mdi:reload" :size="14" />
        <span class="ml-4px">刷新</span>
      </ElButton>
      <ElButton size="small" text @click="resetFilters">重置</ElButton>
      <div class="lore__spacer" />
      <ElButton type="primary" plain size="small" :disabled="!projectUlid" @click="handleCreate">
        <Icon icon="mdi:plus" :size="14" />
        <span class="ml-4px">新建设定</span>
      </ElButton>
    </div>

    <span class="nw-hint">共 {{ loreList.length }} 条设定，世界观、地点、物品与历史事件都在这里。</span>

    <div v-loading="loading" class="lore__grid">
      <ElCard v-for="lore in loreList" :key="lore.lore_ulid" shadow="hover" class="lore-card">
        <div class="lore-card__head">
          <h3 class="lore-card__name" :title="lore.lore_name">{{ lore.lore_name }}</h3>
          <span class="nw-badge nw-badge--neutral">{{ typeLabel(lore.lore_type_code) }}</span>
        </div>
        <p class="lore-card__body">{{ contentSnippet(lore) }}</p>
        <div class="lore-card__meta">
          <span>{{ formatRelativeTime(lore.updated_at) }}</span>
          <div class="lore-card__spacer" />
          <ElButton size="small" text @click="handleEdit(lore)">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="handleDelete(lore)">删除</ElButton>
        </div>
      </ElCard>

      <ElEmpty
        v-if="!loading && loreList.length === 0"
        :description="
          !projectUlid
            ? '请先在顶栏选择项目'
            : keywordText || typeCode
              ? '没有匹配的设定，换个关键词试试'
              : '还没有设定，先写一条世界规则吧'
        "
      />
    </div>

    <LoreDrawer ref="drawer" :project-ulid="projectUlid" @saved="handleSaved" />
  </div>
</template>

<style lang="less" scoped>
  .lore {
    &__search {
      width: 240px;
    }

    &__type {
      width: 140px;
    }

    &__spacer {
      flex: 1;
    }

    &__grid {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      align-items: start;
      min-height: 120px;
    }
  }

  .lore-card {
    :deep(.el-card__body) {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 14px 16px;
    }

    &__head {
      display: flex;
      gap: 8px;
      align-items: center;
      justify-content: space-between;
    }

    &__name {
      min-width: 0;
      overflow: hidden;
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__body {
      min-height: 40px;
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--el-text-color-regular);
      word-break: break-word;
    }

    &__meta {
      display: flex;
      gap: 2px;
      align-items: center;
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__spacer {
      flex: 1;
    }
  }
</style>
