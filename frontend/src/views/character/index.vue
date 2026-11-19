<!--
  角色管理页：角色卡片网格（筛选 + 新建/编辑抽屉 + 删除确认）
  与关系图页签（AntV X6 关系图，见 CharacterGraphTab）。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { deleteCharacter, listCharacters, type NWCharacterVo } from '@/api/character'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import CharacterEditDrawer from './CharacterEditDrawer.vue'
import CharacterGraphTab from './CharacterGraphTab.vue'

const route = useRoute()
const { t } = useI18n()

const projectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 角色类型选项（筛选下拉） */
const roleTypeOptions = ['protagonist', 'antagonist', 'supporting', 'minor']

const activeTab = ref('list')
const characterList = ref<NWCharacterVo[]>([])
const listLoading = ref(false)
const roleTypeFilter = ref('')
const keywordText = ref('')
const editDrawerVisible = ref(false)
const editingCharacter = ref<NWCharacterVo | null>(null)
const graphTabRef = ref<InstanceType<typeof CharacterGraphTab> | null>(null)

/** 角色类型展示文案 */
function roleTypeText(roleTypeCode: string): string {
  switch (roleTypeCode) {
    case 'protagonist':
      return t('character.roleProtagonist')
    case 'antagonist':
      return t('character.roleAntagonist')
    case 'supporting':
      return t('character.roleSupporting')
    case 'minor':
      return t('character.roleMinor')
    default:
      return roleTypeCode
  }
}

/** 加载角色列表（类型/关键词过滤） */
async function loadCharacters(): Promise<void> {
  if (!projectUlid.value) {
    return
  }
  listLoading.value = true
  try {
    const listData = await listCharacters({
      project_ulid: projectUlid.value,
      role_type_code: roleTypeFilter.value || undefined,
      keyword_text: keywordText.value || undefined,
    })
    characterList.value = Array.isArray(listData) ? listData : []
  } catch {
    // 失败提示由 http 层统一弹出
    characterList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 新建角色 */
function handleCreate(): void {
  editingCharacter.value = null
  editDrawerVisible.value = true
}

/** 编辑角色 */
function handleEdit(characterVo: NWCharacterVo): void {
  editingCharacter.value = characterVo
  editDrawerVisible.value = true
}

/** 删除角色（二次确认，连带删关系边） */
async function handleDelete(characterVo: NWCharacterVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('character.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteCharacter(characterVo.character_ulid)
    ElMessage.success(t('character.deleteSuccess'))
    await loadCharacters()
    graphTabRef.value?.reload()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 保存成功：刷新列表与关系图 */
function handleSaved(): void {
  void loadCharacters()
  graphTabRef.value?.reload()
}

watch(activeTab, (tabName) => {
  if (tabName === 'graph') {
    graphTabRef.value?.reload()
  }
})

watch(projectUlid, () => {
  keywordText.value = ''
  roleTypeFilter.value = ''
  void loadCharacters()
})

onMounted(() => {
  void loadCharacters()
})
</script>

<template>
  <div class="nw-page character-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
      :title_text="t('character.title')"
      :subtitle_text="t('character.pageSubtitle')"
      icon_code="users"
    >
      <template #actions>
        <NWButton type="primary" @click="handleCreate">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('character.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <NWCard :hoverable="false">
      <el-tabs v-model="activeTab">
        <!-- 角色列表页签 -->
        <el-tab-pane :label="t('character.listTab')" name="list">
          <div class="character-page__filters">
            <el-select
              v-model="roleTypeFilter"
              clearable
              class="character-page__role-select"
              :placeholder="t('character.roleTypePlaceholder')"
              @change="loadCharacters"
            >
              <el-option
                v-for="roleType in roleTypeOptions"
                :key="roleType"
                :label="roleTypeText(roleType)"
                :value="roleType"
              />
            </el-select>
            <el-input
              v-model="keywordText"
              class="character-page__keyword"
              clearable
              :placeholder="t('character.keywordPlaceholder')"
              @keyup.enter="loadCharacters"
              @clear="loadCharacters"
            />
            <NWButton size="small" @click="loadCharacters">
              <NWIcon icon_code="search" :size_px="14" />
              {{ t('character.search') }}
            </NWButton>
            <span class="nw-toolbar__spacer" />
            <span class="character-page__count">{{ t('character.countLabel', { count: characterList.length }) }}</span>
          </div>

          <NWLoading v-if="listLoading && characterList.length === 0" />
          <NWEmpty
            v-else-if="characterList.length === 0"
            icon_code="users"
            :title_text="t('character.emptyTitle')"
            :description_text="t('character.emptyTip')"
          >
            <template #actions>
              <NWButton size="small" type="primary" @click="handleCreate">{{ t('character.create') }}</NWButton>
            </template>
          </NWEmpty>
          <div v-else class="character-page__grid">
            <article
              v-for="characterVo in characterList"
              :key="characterVo.character_ulid"
              class="character-card"
            >
              <div class="character-card__top">
                <span class="character-card__avatar" :class="`is-${characterVo.role_type_code || 'minor'}`">
                  {{ (characterVo.character_name || '#').charAt(0) }}
                </span>
                <div class="character-card__head">
                  <h4 class="character-card__name" :title="characterVo.character_name">
                    {{ characterVo.character_name }}
                  </h4>
                  <span v-if="characterVo.alias_text" class="character-card__alias" :title="characterVo.alias_text">
                    {{ characterVo.alias_text }}
                  </span>
                </div>
                <span
                  class="character-card__role"
                  :class="`is-${characterVo.role_type_code || 'minor'}`"
                >
                  {{ roleTypeText(characterVo.role_type_code) }}
                </span>
              </div>
              <p class="character-card__personality">
                {{ characterVo.personality_text || t('character.noPersonality') }}
              </p>
              <div class="character-card__actions">
                <NWButton size="small" @click="handleEdit(characterVo)">{{ t('common.edit') }}</NWButton>
                <NWButton size="small" type="danger-text" @click="handleDelete(characterVo)">
                  {{ t('common.delete') }}
                </NWButton>
              </div>
            </article>
          </div>
        </el-tab-pane>

        <!-- 关系图页签 -->
        <el-tab-pane :label="t('character.graphTab')" name="graph">
          <CharacterGraphTab ref="graphTabRef" :project-ulid="projectUlid" />
        </el-tab-pane>
      </el-tabs>
    </NWCard>

    <CharacterEditDrawer
      v-model:visible="editDrawerVisible"
      :project-ulid="projectUlid"
      :character="editingCharacter"
      @saved="handleSaved"
    />
  </div>
</template>

<style scoped src="./character.css"></style>
