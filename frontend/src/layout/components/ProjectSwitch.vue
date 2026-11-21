<!-- 项目切换器：列出可用项目并切换当前项目。 -->
<script setup lang="ts">
  import { computed, onMounted } from 'vue'
  import { useRouter } from 'vue-router'
  import { useProjectStore } from '@/store/modules/project'

  const projectStore = useProjectStore()
  const router = useRouter()

  const options = computed(() => projectStore.projectOptions)
  const currentUlid = computed(() => projectStore.currentProjectUlid)

  /** 切换项目后停留在当前功能页（只替换 URL 上的 project_ulid），没有项目页就回仪表盘 */
  const switchProject = (projectUlid: string) => {
    projectStore.selectProject(projectUlid)
    const currentPath = router.currentRoute.value.path
    const replaced = currentPath.replace(/\/PRJ_[A-Z0-9]+/i, `/${projectUlid}`)
    if (replaced !== currentPath) {
      void router.replace(replaced)
    }
  }

  onMounted(() => {
    if (projectStore.projectOptions.length === 0) void projectStore.loadProjectOptions()
  })
</script>

<template>
  <div class="project-switch">
    <Icon icon="mdi:book-open-variant" :size="17" />
    <el-select
      :model-value="currentUlid"
      class="project-switch__select"
      placeholder="选择项目"
      filterable
      size="small"
      @update:model-value="switchProject"
    >
      <el-option
        v-for="item in options"
        :key="item.project_ulid"
        :label="item.project_name"
        :value="item.project_ulid"
      />
    </el-select>
  </div>
</template>

<style lang="less" scoped>
  .project-switch {
    display: flex;
    gap: 6px;
    align-items: center;
    padding-left: 12px;
    color: var(--top-header-text-color);
    border-left: 1px solid var(--layout-border-color);

    &__select {
      width: 168px;
    }
  }
</style>
