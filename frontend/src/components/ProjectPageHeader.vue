<!--
  项目内页顶栏：返回工作台按钮 + 项目名（GET /project/detail）。
  角色 / 知识库 / 创作笔记三页复用。
-->
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getProjectDetail } from '@/api/project'
import { useProjectStore } from '@/stores/project'
import NWButton from '@/components/NWButton.vue'

const props = defineProps<{
  projectUlid: string
}>()

const router = useRouter()
const { t } = useI18n()
const projectStore = useProjectStore()

const projectName = ref('')
const loading = ref(false)

/** 加载项目名（失败静默，提示由 http 层统一弹出；已删除项目不再发起请求） */
async function loadProjectName(): Promise<void> {
  if (!props.projectUlid) {
    projectName.value = ''
    return
  }
  if (!projectStore.hasProject(props.projectUlid)) {
    // 列表已加载但项目不在其中：已删除或无权访问，交由布局守卫跳转
    if (!projectStore.list_loading) {
      projectName.value = ''
      return
    }
  }
  loading.value = true
  try {
    const projectVo = await getProjectDetail(props.projectUlid)
    projectName.value = projectVo.project_name
  } catch {
    projectName.value = ''
  } finally {
    loading.value = false
  }
}

function handleBack(): void {
  void router.push(`/workspace/${props.projectUlid}`)
}

watch(
  () => props.projectUlid,
  () => {
    void loadProjectName()
  },
)

onMounted(() => {
  void loadProjectName()
})

defineExpose({ loading })
</script>

<template>
  <header class="project-page-header">
    <NWButton size="small" @click="handleBack">
      {{ t('projectPage.backToWorkspace') }}
    </NWButton>
    <h2 class="project-page-header__name" :title="projectName">
      {{ projectName }}
    </h2>
  </header>
</template>

<style scoped>
.project-page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 12px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
}

.project-page-header__name {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
