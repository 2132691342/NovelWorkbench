<!--
  世界观设定页：左侧设定分类（全部/世界规则/地点/物品/事件）+ 关键词搜索，右侧设定卡片列表与编辑抽屉。
  设定按 sort_order 升序展示；AI 对话会自动注入这些设定作为上下文（创作笔记类型不注入）。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { deleteLore, saveLore, type NWLoreVo } from '@/api/lore'
import { post } from '@/api/http'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'

const route = useRoute()
const { t } = useI18n()

const projectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 设定类型（note 为创作笔记，不在此页展示） */
const NW_LORE_TYPE_LIST = ['world', 'rule', 'place', 'item', 'event'] as const

/** 设定类型展示文案 */
function loreTypeText(loreTypeCode: string): string {
  switch (loreTypeCode) {
    case 'world':
      return t('lore.typeWorld')
    case 'rule':
      return t('lore.typeRule')
    case 'place':
      return t('lore.typePlace')
    case 'item':
      return t('lore.typeItem')
    case 'event':
      return t('lore.typeEvent')
    default:
      return loreTypeCode
  }
}

/** 设定类型标签色 */
function loreTypeTagClass(loreTypeCode: string): string {
  return `lore-tag lore-tag--${loreTypeCode}`
}

interface LoreFormState {
  lore_name: string
  lore_type_code: string
  content_text: string
  sort_order: number
}

const loreList = ref<NWLoreVo[]>([])
const listLoading = ref(false)
const activeTypeCode = ref('')
const keywordText = ref('')
const drawerVisible = ref(false)
const savingFlag = ref(false)
const editingUlid = ref('')
const formRef = ref<FormInstance>()

const formState = reactive<LoreFormState>({
  lore_name: '',
  lore_type_code: 'world',
  content_text: '',
  sort_order: 0,
})

const formRules = computed<FormRules>(() => ({
  lore_name: [
    { required: true, message: t('lore.ruleNameRequired'), trigger: 'blur' },
    { max: 128, message: t('lore.ruleNameMax'), trigger: 'blur' },
  ],
}))

/** 类型计数（侧栏徽标） */
const typeCountMap = computed<Record<string, number>>(() => {
  const countMap: Record<string, number> = { '': loreList.value.length }
  for (const loreItem of loreList.value) {
    const typeCode = loreItem.lore_type_code || 'world'
    countMap[typeCode] = (countMap[typeCode] ?? 0) + 1
  }
  return countMap
})

/** 过滤后的设定列表（服务端已按类型过滤，这里只做前端即时搜索） */
const filteredLoreList = computed<NWLoreVo[]>(() => {
  const keyword = keywordText.value.trim().toLowerCase()
  if (!keyword) {
    return loreList.value
  }
  return loreList.value.filter(
    (loreItem) =>
      loreItem.lore_name.toLowerCase().includes(keyword) ||
      loreItem.content_text.toLowerCase().includes(keyword),
  )
})

/** 加载设定列表（服务端按类型过滤） */
async function loadLoreList(): Promise<void> {
  if (!projectUlid.value) {
    return
  }
  listLoading.value = true
  try {
    const list = await post<NWLoreVo[]>('/lore/list', {
      project_ulid: projectUlid.value,
      lore_type_code: activeTypeCode.value || undefined,
    })
    loreList.value = Array.isArray(list) ? list : []
  } catch {
    loreList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 切换类型筛选 */
function handleTypeSelect(loreTypeCode: string): void {
  if (activeTypeCode.value === loreTypeCode) {
    return
  }
  activeTypeCode.value = loreTypeCode
  void loadLoreList()
}

/** 打开新建抽屉 */
function handleCreate(): void {
  editingUlid.value = ''
  formState.lore_name = ''
  formState.lore_type_code = activeTypeCode.value || 'world'
  formState.content_text = ''
  formState.sort_order = loreList.value.length
  drawerVisible.value = true
}

/** 打开编辑抽屉 */
function handleEdit(loreItem: NWLoreVo): void {
  editingUlid.value = loreItem.lore_ulid
  formState.lore_name = loreItem.lore_name
  formState.lore_type_code = loreItem.lore_type_code || 'world'
  formState.content_text = loreItem.content_text
  formState.sort_order = loreItem.sort_order
  drawerVisible.value = true
}

/** 保存设定（新建或更新） */
async function handleSubmit(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  savingFlag.value = true
  try {
    await saveLore({
      lore_ulid: editingUlid.value || undefined,
      project_ulid: projectUlid.value,
      lore_name: formState.lore_name,
      lore_type_code: formState.lore_type_code,
      content_text: formState.content_text,
      sort_order: formState.sort_order,
    })
    ElMessage.success(t('lore.saveSuccess'))
    drawerVisible.value = false
    await loadLoreList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingFlag.value = false
  }
}

/** 删除设定（二次确认） */
async function handleDelete(loreItem: NWLoreVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('lore.deleteConfirm'), t('common.delete'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteLore(loreItem.lore_ulid)
    ElMessage.success(t('lore.deleteSuccess'))
    await loadLoreList()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

watch(projectUlid, () => {
  activeTypeCode.value = ''
  keywordText.value = ''
  void loadLoreList()
})

onMounted(() => {
  void loadLoreList()
})
</script>

<template>
  <div class="nw-page lore-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
      :title_text="t('lore.title')"
      :subtitle_text="t('lore.pageSubtitle')"
      icon_code="globe"
    >
      <template #actions>
        <NWButton type="primary" @click="handleCreate">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('lore.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="lore-page__body">
      <aside class="lore-page__side">
        <NWCard :title="t('lore.typeTitle')" :hoverable="false">
          <ul class="lore-type-list">
            <li
              class="lore-type-item"
              :class="{ 'is-active': activeTypeCode === '' }"
              @click="handleTypeSelect('')"
            >
              <span>{{ t('lore.typeAll') }}</span>
              <span class="lore-type-count">{{ typeCountMap[''] ?? 0 }}</span>
            </li>
            <li
              v-for="loreTypeCode in NW_LORE_TYPE_LIST"
              :key="loreTypeCode"
              class="lore-type-item"
              :class="{ 'is-active': activeTypeCode === loreTypeCode }"
              @click="handleTypeSelect(loreTypeCode)"
            >
              <span>{{ loreTypeText(loreTypeCode) }}</span>
              <span class="lore-type-count">{{ typeCountMap[loreTypeCode] ?? 0 }}</span>
            </li>
          </ul>
        </NWCard>
      </aside>

      <section class="lore-page__main">
        <NWCard :hoverable="false">
          <template #header>
            <h3 class="nw-card__title">{{ t('lore.entriesTitle') }}</h3>
            <div class="lore-page__toolbar">
              <el-input
                v-model="keywordText"
                :placeholder="t('lore.searchPlaceholder')"
                clearable
                class="lore-page__search"
              >
                <template #prefix><NWIcon icon_code="search" :size_px="14" /></template>
              </el-input>
              <span class="lore-page__count">{{ t('lore.countLabel', { count: filteredLoreList.length }) }}</span>
            </div>
          </template>

          <NWLoading v-if="listLoading" />
          <NWEmpty
            v-else-if="filteredLoreList.length === 0"
            icon_code="globe"
            :title_text="t('lore.emptyTitle')"
            :description_text="t('lore.emptyTip')"
          >
            <template #actions>
              <NWButton size="small" type="primary" @click="handleCreate">{{ t('lore.create') }}</NWButton>
            </template>
          </NWEmpty>
          <div v-else class="lore-grid">
            <article
              v-for="loreItem in filteredLoreList"
              :key="loreItem.lore_ulid"
              class="lore-card"
            >
              <header class="lore-card__header">
                <h4 class="lore-card__name" :title="loreItem.lore_name">
                  {{ loreItem.lore_name }}
                </h4>
                <span :class="loreTypeTagClass(loreItem.lore_type_code)">
                  {{ loreTypeText(loreItem.lore_type_code) }}
                </span>
              </header>
              <p class="lore-card__content">{{ loreItem.content_text || t('lore.noContent') }}</p>
              <footer class="lore-card__footer">
                <span class="lore-card__time">
                  {{ new Date(loreItem.updated_at_ms).toLocaleString() }}
                </span>
                <span class="lore-card__actions">
                  <NWButton size="small" @click="handleEdit(loreItem)">{{ t('common.edit') }}</NWButton>
                  <NWButton type="danger-text" size="small" @click="handleDelete(loreItem)">
                    {{ t('common.delete') }}
                  </NWButton>
                </span>
              </footer>
            </article>
          </div>
        </NWCard>
      </section>
    </div>

    <!-- 设定编辑抽屉 -->
    <el-drawer
      v-model="drawerVisible"
      :title="editingUlid ? t('lore.editTitle') : t('lore.createTitle')"
      size="520px"
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <el-form-item :label="t('lore.loreName')" prop="lore_name">
          <el-input v-model="formState.lore_name" maxlength="128" show-word-limit />
        </el-form-item>
        <el-form-item :label="t('lore.loreType')">
          <el-select v-model="formState.lore_type_code" class="lore-drawer__select">
            <el-option
              v-for="loreTypeCode in NW_LORE_TYPE_LIST"
              :key="loreTypeCode"
              :value="loreTypeCode"
              :label="loreTypeText(loreTypeCode)"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('lore.contentText')">
          <el-input
            v-model="formState.content_text"
            type="textarea"
            :rows="12"
            maxlength="20000"
            show-word-limit
            :placeholder="t('lore.contentPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('lore.sortOrder')">
          <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="savingFlag" @click="handleSubmit">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped src="./lore.css"></style>
