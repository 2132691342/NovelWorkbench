<!--
  输入区底部工具栏：提示词档案、历史条数、@ 引用入口、发送/停止。
  纯展示与事件上抛，输入态与发送逻辑仍由 ChatComposer 持有。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWPromptProfileVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'

defineProps<{
  profileList: NWPromptProfileVo[]
  promptProfileUlid: string
  historyCount: number
  canSendFlag: boolean
  streamingFlag: boolean
}>()

const emit = defineEmits<{
  'update:promptProfileUlid': [promptProfileUlid: string]
  'update:historyCount': [historyCount: number]
  openRefPicker: []
  send: []
  stop: []
}>()

const { t } = useI18n()

/** 历史条数选项（0 = 使用全局默认） */
const historyOptions = computed(() => [
  { value: 0, label: t('workspace.ai.composer.historyDefault') },
  { value: 1, label: '1' },
  { value: 3, label: '3' },
  { value: 5, label: '5' },
  { value: 10, label: '10' },
])
</script>

<template>
  <div class="chat-composer__toolbar">
    <el-tooltip v-if="profileList.length > 0" :content="t('workspace.ai.composer.profileHint')" placement="top">
      <el-select
        :model-value="promptProfileUlid"
        size="small"
        class="chat-composer__profile-select"
        clearable
        :placeholder="t('workspace.ai.composer.profileDefault')"
        @update:model-value="emit('update:promptProfileUlid', $event ?? '')"
      >
        <el-option
          v-for="profileItem in profileList"
          :key="profileItem.prompt_profile_ulid"
          :label="profileItem.prompt_profile_name"
          :value="profileItem.prompt_profile_ulid"
        />
      </el-select>
    </el-tooltip>

    <el-tooltip :content="t('workspace.ai.composer.historyHint')" placement="top">
      <label class="chat-composer__history">
        <NWIcon icon_code="history" :size_px="13" />
        <el-select
          :model-value="historyCount"
          size="small"
          class="chat-composer__history-select"
          @update:model-value="emit('update:historyCount', $event)"
        >
          <el-option
            v-for="optionItem in historyOptions"
            :key="optionItem.value"
            :label="optionItem.label"
            :value="optionItem.value"
          />
        </el-select>
      </label>
    </el-tooltip>

    <button type="button" class="chat-composer__tool-btn" :disabled="streamingFlag" @click="emit('openRefPicker')">
      <NWIcon icon_code="atSign" :size_px="14" />
      {{ t('workspace.ai.composer.atButton') }}
    </button>

    <span class="chat-composer__hint">Ctrl + Enter</span>

    <button
      v-if="!streamingFlag"
      type="button"
      class="chat-composer__send-btn"
      :disabled="!canSendFlag"
      @click="emit('send')"
    >
      <NWIcon icon_code="send" :size_px="14" />
      {{ t('workspace.ai.composer.send') }}
    </button>
    <button v-else type="button" class="chat-composer__send-btn is-stop" @click="emit('stop')">
      <span class="chat-composer__stop-icon" />
      {{ t('workspace.ai.composer.stop') }}
    </button>
  </div>
</template>

<style scoped src="./ComposerToolbar.css"></style>
