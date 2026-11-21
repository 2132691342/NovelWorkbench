<!-- 技能试用弹窗：解析技能并展示缺失变量。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElDialog,
    ElEmpty,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    ElTag
  } from 'element-plus'
  import { resolveSkill } from '@/api/skill'
  import type { NWSkillResolveVo, NWSkillVo } from '@/api/contracts'

  const props = defineProps<{ projectUlid: string }>()

  const visible = ref(false)
  const running = ref(false)
  const skill = ref<NWSkillVo | null>(null)
  const contentText = ref('')
  const result = ref<NWSkillResolveVo | null>(null)

  const variableEntries = computed(() =>
    Object.entries(result.value?.variable_map ?? {}).filter(
      ([key]) => key !== 'skill_code' && key !== 'user_input'
    )
  )

  /** 打开试用弹窗：自动填入斜杠命令前缀 */
  const open = (target: NWSkillVo) => {
    skill.value = target
    contentText.value = `/${target.skill_code} `
    result.value = null
    visible.value = true
  }

  const run = async () => {
    if (!skill.value) return
    if (!props.projectUlid) {
      ElMessage.warning('请先在顶栏选择一个项目，技能试用需要项目上下文')
      return
    }
    if (!contentText.value.trim()) {
      ElMessage.warning('请填写试用内容')
      return
    }
    running.value = true
    try {
      const text = contentText.value.trim()
      const slashPrefix = `/${skill.value.skill_code}`
      // 以斜杠命令开头时交给后端解析（会剥离命令），否则显式指定技能编码
      const explicitCode = text.startsWith(slashPrefix) ? undefined : skill.value.skill_code
      result.value = await resolveSkill({
        project_ulid: props.projectUlid,
        content_text: contentText.value,
        skill_code: explicitCode
      })
    } finally {
      running.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog
    v-model="visible"
    :title="skill ? `试用技能 · ${skill.skill_name}` : '试用技能'"
    width="620px"
    append-to-body
    destroy-on-close
  >
    <ElForm label-position="top">
      <ElFormItem label="输入内容">
        <ElInput
          v-model="contentText"
          type="textarea"
          :rows="4"
          maxlength="20000"
          placeholder="以 /技能编码 开头，后面跟正文，例如：/polish 请润色这段雨夜追车"
        />
      </ElFormItem>
    </ElForm>

    <div class="trial__actions">
      <span class="nw-hint">解析会命中参考资料并计算变量取值，不会真的调用模型。</span>
      <div class="trial__spacer" />
      <ElButton type="primary" plain :loading="running" @click="run">解析</ElButton>
    </div>

    <div v-if="result" class="trial__result">
      <ElAlert
        v-if="result.missing_list.length > 0"
        type="warning"
        :closable="false"
        show-icon
        :title="`缺少必填变量：${result.missing_list.join('、')}`"
      />
      <div class="trial__block">
        <span class="trial__label">命中技能</span>
        <div class="trial__tags">
          <ElTag v-if="result.skill_code" size="small" effect="plain" type="success">
            {{ result.skill_name }}（/{{ result.skill_code }}）
          </ElTag>
          <span v-else class="nw-hint">未命中技能，按自由对话处理</span>
        </div>
      </div>
      <div class="trial__block">
        <span class="trial__label">剩余正文</span>
        <p class="trial__text">{{ result.remainder_text || '（空）' }}</p>
      </div>
      <div class="trial__block">
        <span class="trial__label">变量取值</span>
        <div v-if="variableEntries.length > 0" class="trial__tags">
          <ElTag v-for="[key, value] in variableEntries" :key="key" size="small" effect="plain">
            {{ key }} = {{ value || '（空）' }}
          </ElTag>
        </div>
        <span v-else class="nw-hint">没有额外变量</span>
      </div>
      <div class="trial__block">
        <span class="trial__label">命中参考资料（{{ result.ref_text_list.length }}）</span>
        <ElEmpty v-if="result.ref_text_list.length === 0" description="没有命中资料" :image-size="48" />
        <pre v-for="(text, index) in result.ref_text_list" :key="index" class="trial__pre">{{
          text
        }}</pre>
      </div>
    </div>
    <ElEmpty v-else description="输入内容后点「解析」查看渲染结果" :image-size="64" />
  </ElDialog>
</template>

<style lang="less" scoped>
  .trial {
    &__actions {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    &__spacer {
      flex: 1;
    }

    &__result {
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding-top: 12px;
      margin-top: 12px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &__block {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    &__label {
      font-size: 12px;
      font-weight: 600;
      color: var(--nw-text-secondary);
    }

    &__text {
      padding: 8px 10px;
      font-size: 13px;
      color: var(--nw-ink);
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
      word-break: break-word;
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    &__pre {
      max-height: 160px;
      padding: 10px;
      overflow: auto;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 12px;
      line-height: 1.7;
      color: var(--nw-text-secondary);
      white-space: pre-wrap;
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
    }
  }
</style>
