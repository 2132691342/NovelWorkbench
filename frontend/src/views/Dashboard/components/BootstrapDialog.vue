<!-- AI 开书向导：一句创意 → 方案预览与微调 → 一次性落库。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { applyBootstrapPlan, generateBootstrapPlan, type NWBootstrapPlanVo } from '@/api/bootstrap'
  import type { NWProjectVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'

  const emit = defineEmits<{ created: [project: NWProjectVo] }>()

  const projectStore = useProjectStore()

  const visible = ref(false)
  const step = ref<0 | 1>(0)
  const generating = ref(false)
  const saving = ref(false)
  const plan = ref<NWBootstrapPlanVo | null>(null)

  const form = reactive({
    idea_text: '',
    genre_code: '',
    target_chapter_count: 12
  })

  const genreOptions = [
    { value: '', label: '由 AI 判断' },
    { value: 'fantasy', label: '玄幻' },
    { value: 'scifi', label: '科幻' },
    { value: 'romance', label: '言情' },
    { value: 'suspense', label: '悬疑' },
    { value: 'wuxia', label: '武侠' },
    { value: 'urban', label: '都市' },
    { value: 'history', label: '历史' }
  ]

  const totalChapters = computed(
    () => plan.value?.volume_list.reduce((sum, volume) => sum + volume.chapter_list.length, 0) ?? 0
  )

  /** 打开向导并重置状态 */
  const open = () => {
    visible.value = true
    step.value = 0
    plan.value = null
  }

  const generate = async () => {
    if (form.idea_text.trim().length < 6) {
      ElMessage.warning('先用一句话说清这本书写什么（至少 6 个字）')
      return
    }
    generating.value = true
    try {
      plan.value = await generateBootstrapPlan({
        idea_text: form.idea_text.trim(),
        genre_code: form.genre_code || undefined,
        target_chapter_count: form.target_chapter_count
      })
      step.value = 1
    } finally {
      generating.value = false
    }
  }

  /** 重新生成：保留创意与题材，只换一版方案 */
  const regenerate = () => {
    plan.value = null
    step.value = 0
    void generate()
  }

  const apply = async () => {
    if (!plan.value) return
    saving.value = true
    try {
      const result = await applyBootstrapPlan(plan.value)
      ElMessage.success(`已建《${result.project_vo.project_name}》：${result.chapter_count} 章 / ${result.character_count} 角色 / ${result.lore_count} 设定`)
      visible.value = false
      await projectStore.loadProjectOptions()
      emit('created', result.project_vo)
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" title="AI 开书" width="720px" append-to-body destroy-on-close>
    <ElSteps :active="step" simple class="bootstrap__steps">
      <ElStep title="说一句创意" />
      <ElStep title="确认方案并落库" />
    </ElSteps>

    <div v-if="step === 0" class="bootstrap__form">
      <ElInput
        v-model="form.idea_text"
        type="textarea"
        :rows="4"
        maxlength="2000"
        show-word-limit
        placeholder="例如：一个被宗门当成废物的外门弟子，捡到一只能催熟灵药的小瓶，必须在三年内筑基，否则母亲会被逐出山门"
      />
      <div class="bootstrap__row">
        <ElSelect v-model="form.genre_code" class="bootstrap__select" placeholder="题材">
          <ElOption v-for="item in genreOptions" :key="item.value" :label="item.label" :value="item.value" />
        </ElSelect>
        <ElInputNumber v-model="form.target_chapter_count" :min="3" :max="60" controls-position="right" />
        <span class="nw-hint">章节数，默认 12 章</span>
      </div>
      <p class="nw-hint">
        生成的是可编辑的方案：书名、卖点、卷章、角色、世界观都会先给你看，确认后才写入项目。
      </p>
    </div>

    <div v-else-if="plan" class="bootstrap__plan">
      <ElForm label-position="top">
        <ElFormItem label="书名">
          <ElInput v-model="plan.project_name" maxlength="80" />
        </ElFormItem>
        <ElFormItem label="卖点">
          <ElInput v-model="plan.selling_point_text" type="textarea" :rows="2" />
        </ElFormItem>
        <ElFormItem label="简介">
          <ElInput v-model="plan.synopsis_text" type="textarea" :rows="3" />
        </ElFormItem>
        <ElFormItem label="文风约束">
          <ElInput v-model="plan.style_text" type="textarea" :rows="2" />
        </ElFormItem>
      </ElForm>

      <div class="bootstrap__section">
        <div class="bootstrap__section-head">
          <strong>卷章结构</strong>
          <span class="nw-hint">
            {{ plan.volume_list.length }} 卷 / {{ totalChapters }} 章
          </span>
        </div>
        <ElCollapse class="bootstrap__volumes">
          <ElCollapseItem
            v-for="(volume, volumeIndex) in plan.volume_list"
            :key="volumeIndex"
            :name="String(volumeIndex)"
          >
            <template #title>
              <span class="bootstrap__volume-title">{{ volume.volume_name }}</span>
            </template>
            <ElInput v-model="volume.volume_name" class="bootstrap__field" placeholder="卷名" />
            <ElInput
              v-model="volume.summary_text"
              class="bootstrap__field"
              type="textarea"
              :rows="2"
              placeholder="本卷从哪到哪、卷末翻什么牌"
            />
            <div
              v-for="(chapter, chapterIndex) in volume.chapter_list"
              :key="chapterIndex"
              class="bootstrap__chapter"
            >
              <ElInput v-model="chapter.chapter_name" class="bootstrap__chapter-name" placeholder="章节名" />
              <ElInput
                v-model="chapter.summary_text"
                type="textarea"
                :rows="2"
                placeholder="发生什么 / 章末停在哪"
              />
            </div>
          </ElCollapseItem>
        </ElCollapse>
      </div>

      <div class="bootstrap__section">
        <div class="bootstrap__section-head">
          <strong>角色</strong>
          <span class="nw-hint">{{ plan.character_list.length }} 个</span>
        </div>
        <div v-for="(character, index) in plan.character_list" :key="index" class="bootstrap__card">
          <ElInput v-model="character.character_name" class="bootstrap__card-name" placeholder="姓名" />
          <ElSelect v-model="character.role_type_code" class="bootstrap__card-role">
            <ElOption label="主角" value="protagonist" />
            <ElOption label="对手" value="antagonist" />
            <ElOption label="配角" value="supporting" />
            <ElOption label="龙套" value="minor" />
          </ElSelect>
          <ElInput v-model="character.goal_text" class="bootstrap__card-field" placeholder="目标动机" />
          <ElInput v-model="character.personality_text" class="bootstrap__card-field" placeholder="性格" />
        </div>
      </div>

      <div class="bootstrap__section">
        <div class="bootstrap__section-head">
          <strong>世界观设定</strong>
          <span class="nw-hint">{{ plan.lore_list.length }} 条</span>
        </div>
        <div v-for="(lore, index) in plan.lore_list" :key="index" class="bootstrap__card">
          <ElInput v-model="lore.lore_name" class="bootstrap__card-name" placeholder="设定名" />
          <ElSelect v-model="lore.lore_type_code" class="bootstrap__card-role">
            <ElOption label="世界" value="world" />
            <ElOption label="规则" value="rule" />
            <ElOption label="物品" value="item" />
            <ElOption label="地点" value="place" />
            <ElOption label="事件" value="event" />
          </ElSelect>
          <ElInput
            v-model="lore.content_text"
            class="bootstrap__card-field"
            type="textarea"
            :rows="2"
            placeholder="规则 / 边界 / 代价"
          />
        </div>
      </div>
    </div>

    <template #footer>
      <ElButton v-if="step === 0" @click="visible = false">取消</ElButton>
      <ElButton v-if="step === 0" type="primary" :loading="generating" @click="generate">
        {{ generating ? '正在构思…' : '生成方案' }}
      </ElButton>
      <template v-if="step === 1">
        <ElButton @click="step = 0">返回修改创意</ElButton>
        <ElButton :loading="generating" @click="regenerate">换一版</ElButton>
        <ElButton type="primary" :loading="saving" @click="apply">确认开书</ElButton>
      </template>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .bootstrap {
    &__steps {
      margin-bottom: 16px;
    }

    &__form {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    &__row {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    &__select {
      width: 160px;
    }

    &__plan {
      display: flex;
      flex-direction: column;
      gap: 16px;
      max-height: 56vh;
      padding-right: 6px;
      overflow-y: auto;
    }

    &__section {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    &__section-head {
      display: flex;
      gap: 10px;
      align-items: baseline;
      justify-content: space-between;
      padding-bottom: 6px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__volume-title {
      font-weight: 600;
    }

    &__field {
      margin-bottom: 8px;
    }

    &__chapter {
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 8px 0 8px 10px;
      border-left: 2px solid var(--nw-border-soft);
    }

    &__chapter-name {
      max-width: 320px;
    }

    &__card {
      display: grid;
      gap: 8px;
      padding: 10px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
      grid-template-columns: 180px 120px 1fr;
    }

    &__card-name,
    &__card-role {
      width: 100%;
    }

    &__card-field {
      grid-column: 1 / -1;
    }
  }
</style>
