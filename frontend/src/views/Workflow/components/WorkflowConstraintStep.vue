<!-- 向导第 3 步「定约束」：字数、体检分、审核开关、章节范围与额外要求。
     章节手选（搜索/全选/反选）较重，独立成组件，避免向导主文件过长。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import type { NWChapterBriefVo } from '@/api/contracts'

  const props = defineProps<{
    /** 约束表单状态：由向导持有，子组件直接改字段（不整体替换对象） */
    state: {
      targetWordCount: number
      gateScore: number
      reviewGateFlag: boolean
      extraRequirementText: string
      chapterMode: 'blank' | 'pick' | 'new'
      newChapterCount: number
      pickedChapterUlidList: string[]
    }
    /** 可手选的章节（已按模板用途过滤） */
    pickableChapterList: NWChapterBriefVo[]
    blankChapterCount: number
    /** 打磨/去 AI 味模板只能选已有正文的章节 */
    needsWrittenChapter: boolean
  }>()

  const targetWordPresetList = [1500, 2500, 3500, 5000]

  const chapterKeyword = ref('')

  const filteredChapterList = computed(() => {
    const keyword = chapterKeyword.value.trim()
    if (!keyword) return props.pickableChapterList
    return props.pickableChapterList.filter(
      (chapter) => chapter.chapter_name.includes(keyword) || String(chapter.chapter_order).includes(keyword)
    )
  })

  const allFilteredPicked = computed(
    () =>
      filteredChapterList.value.length > 0 &&
      filteredChapterList.value.every((chapter) => props.state.pickedChapterUlidList.includes(chapter.chapter_ulid))
  )

  const chapterSummaryText = computed(() => {
    if (props.state.chapterMode === 'blank') {
      return props.blankChapterCount > 0
        ? `将处理 ${props.blankChapterCount} 个空白章节（按目录顺序逐章执行）`
        : '当前没有空白章节，请改用手动选择或新建章节'
    }
    if (props.state.chapterMode === 'new') return `将新建 ${props.state.newChapterCount} 个空白章节并逐章执行`
    return `已选 ${props.state.pickedChapterUlidList.length} 个章节`
  })

  /** 全选/取消全选只作用于当前搜索结果，避免误选看不见的章节 */
  const toggleAllFiltered = () => {
    const filteredUlidList = filteredChapterList.value.map((chapter) => chapter.chapter_ulid)
    if (allFilteredPicked.value) {
      props.state.pickedChapterUlidList = props.state.pickedChapterUlidList.filter(
        (ulid) => !filteredUlidList.includes(ulid)
      )
      return
    }
    props.state.pickedChapterUlidList = [...new Set([...props.state.pickedChapterUlidList, ...filteredUlidList])]
  }

  const invertFiltered = () => {
    const filteredUlidList = filteredChapterList.value.map((chapter) => chapter.chapter_ulid)
    const restUlidList = props.state.pickedChapterUlidList.filter((ulid) => !filteredUlidList.includes(ulid))
    const invertedUlidList = filteredUlidList.filter((ulid) => !props.state.pickedChapterUlidList.includes(ulid))
    props.state.pickedChapterUlidList = [...restUlidList, ...invertedUlidList]
  }
</script>

<template>
  <div class="wf-wizard__pane">
    <div class="wf-wizard__field">
      <div class="wf-wizard__label">
        <span>每章目标字数</span>
        <strong>{{ state.targetWordCount }} 字</strong>
      </div>
      <ElSlider v-model="state.targetWordCount" :min="300" :max="20000" :step="100" :show-tooltip="false" />
      <div class="wf-wizard__presets">
        <ElButton
          v-for="preset in targetWordPresetList"
          :key="preset"
          size="small"
          :type="state.targetWordCount === preset ? 'primary' : 'default'"
          plain
          @click="state.targetWordCount = preset"
        >
          {{ preset }}
        </ElButton>
      </div>
    </div>

    <div class="wf-wizard__row">
      <label class="wf-wizard__field wf-wizard__field--half">
        <div class="wf-wizard__label">
          <span>体检通过分</span>
          <strong>{{ state.gateScore }} 分</strong>
        </div>
        <ElSlider v-model="state.gateScore" :min="60" :max="100" :step="1" :show-tooltip="false" />
      </label>
      <label class="wf-wizard__field wf-wizard__field--half">
        <div class="wf-wizard__label">
          <span>人工审核门禁</span>
          <ElSwitch v-model="state.reviewGateFlag" />
        </div>
        <p class="nw-hint">
          {{
            state.reviewGateFlag
              ? '每章停在待审核，你通过后才入库并继续下一章'
              : 'AI 直通入库，不等人（适合批量清洗）'
          }}
        </p>
      </label>
    </div>

    <div class="wf-wizard__field">
      <div class="wf-wizard__label"><span>章节范围</span></div>
      <ElRadioGroup v-model="state.chapterMode" class="wf-wizard__modes">
        <ElRadioButton v-if="!needsWrittenChapter" value="blank">
          空白章自动（{{ blankChapterCount }}）
        </ElRadioButton>
        <ElRadioButton value="pick">手选章节（{{ pickableChapterList.length }}）</ElRadioButton>
        <ElRadioButton v-if="!needsWrittenChapter" value="new">新建章节</ElRadioButton>
      </ElRadioGroup>
      <p class="wf-wizard__hint">{{ chapterSummaryText }}</p>

      <div v-if="state.chapterMode === 'new'" class="wf-wizard__row">
        <span class="wf-wizard__hint">新建</span>
        <ElInputNumber v-model="state.newChapterCount" :min="1" :max="50" size="small" />
        <span class="wf-wizard__hint">个空白章节（挂到最后一卷）</span>
      </div>

      <div v-if="state.chapterMode === 'pick'" class="wf-picker">
        <div class="wf-picker__toolbar">
          <ElInput
            v-model="chapterKeyword"
            placeholder="搜索章节名 / 序号"
            clearable
            size="small"
            class="wf-picker__search"
          />
          <ElButton size="small" text @click="toggleAllFiltered()">
            {{ allFilteredPicked ? '取消全选' : '全选' }}
          </ElButton>
          <ElButton size="small" text @click="invertFiltered()">反选</ElButton>
          <span class="nw-hint">已选 {{ state.pickedChapterUlidList.length }} 章</span>
        </div>
        <ElScrollbar max-height="180px">
          <ElCheckboxGroup v-model="state.pickedChapterUlidList" class="wf-picker__list">
            <ElCheckbox
              v-for="chapter in filteredChapterList"
              :key="chapter.chapter_ulid"
              :value="chapter.chapter_ulid"
              class="wf-picker__item"
            >
              第 {{ chapter.chapter_order }} 章 {{ chapter.chapter_name }}
              <span class="wf-picker__words">{{ chapter.word_count }} 字</span>
            </ElCheckbox>
          </ElCheckboxGroup>
          <p v-if="filteredChapterList.length === 0" class="wf-picker__empty">
            {{ needsWrittenChapter ? '项目内还没有写好的章节，请先写章节或改用「逐章成稿」' : '没有匹配的章节' }}
          </p>
        </ElScrollbar>
      </div>
    </div>

    <div class="wf-wizard__field">
      <div class="wf-wizard__label"><span>额外要求（可选）</span></div>
      <ElInput
        v-model="state.extraRequirementText"
        type="textarea"
        :rows="3"
        maxlength="1000"
        show-word-limit
        placeholder="例如「每章至少一次反转，主角不能开挂，结尾必须留钩子」"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  @import './workflowWizard.less';
</style>
