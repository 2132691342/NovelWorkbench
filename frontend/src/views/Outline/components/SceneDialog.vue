<!-- 场景卡弹窗：维护场景的任务单信息。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDialog,
    ElForm,
    ElFormItem,
    ElInput,
    ElInputNumber,
    ElOption,
    ElSelect
  } from 'element-plus'
  import { listCharacters } from '@/api/character'
  import type { NWCharacterVo } from '@/api/character'
  import { saveDetailedScene } from '@/api/outline'
  import type { NWDetailedSceneSaveReqVo, NWDetailedSceneVo, NWOutlinePaceCode } from '@/api/outline'
  import { PACE_OPTIONS } from './meta'

  const props = defineProps<{ projectUlid: string; outlineUlid: string }>()

  const emit = defineEmits<{ saved: [scene: NWDetailedSceneVo] }>()

  const visible = ref(false)
  const saving = ref(false)
  const editingUlid = ref('')
  const characterOptions = ref<NWCharacterVo[]>([])

  const form = reactive<{
    character_ulid_list: string[]
    pace_code: NWOutlinePaceCode | ''
    estimated_words: number
    location_text: string
    conflict_text: string
    entry_state_text: string
    goal_text: string
    key_choice_text: string
    outcome_text: string
    exit_state_text: string
  }>({
    character_ulid_list: [],
    pace_code: '',
    estimated_words: 2000,
    location_text: '',
    conflict_text: '',
    entry_state_text: '',
    goal_text: '',
    key_choice_text: '',
    outcome_text: '',
    exit_state_text: ''
  })

  const title = computed(() => (editingUlid.value ? '编辑场景卡' : '新建场景卡'))

  /** 角色选项按项目加载，失败时保持空列表不阻塞编辑 */
  const loadCharacters = async () => {
    if (!props.projectUlid) return
    try {
      characterOptions.value = await listCharacters({ project_ulid: props.projectUlid })
    } catch {
      characterOptions.value = []
    }
  }

  /** 打开弹窗：传 scene 为编辑，不传为新建 */
  const open = (scene?: NWDetailedSceneVo) => {
    editingUlid.value = scene?.scene_ulid ?? ''
    form.character_ulid_list = [...(scene?.character_ulid_list ?? [])]
    form.pace_code = scene?.pace_code ?? ''
    form.estimated_words = scene?.estimated_words ?? 2000
    form.location_text = scene?.location_text ?? ''
    form.conflict_text = scene?.conflict_text ?? ''
    form.entry_state_text = scene?.entry_state_text ?? ''
    form.goal_text = scene?.goal_text ?? ''
    form.key_choice_text = scene?.key_choice_text ?? ''
    form.outcome_text = scene?.outcome_text ?? ''
    form.exit_state_text = scene?.exit_state_text ?? ''
    visible.value = true
    void loadCharacters()
  }

  const submit = async () => {
    if (!props.outlineUlid) return
    saving.value = true
    try {
      const payload: NWDetailedSceneSaveReqVo = {
        project_ulid: props.projectUlid,
        outline_ulid: props.outlineUlid,
        character_ulid_list: form.character_ulid_list,
        pace_code: form.pace_code,
        estimated_words: form.estimated_words,
        location_text: form.location_text,
        conflict_text: form.conflict_text,
        entry_state_text: form.entry_state_text,
        goal_text: form.goal_text,
        key_choice_text: form.key_choice_text,
        outcome_text: form.outcome_text,
        exit_state_text: form.exit_state_text
      }
      if (editingUlid.value) payload.scene_ulid = editingUlid.value
      const savedScene = await saveDetailedScene(payload)
      visible.value = false
      emit('saved', savedScene)
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="640px" append-to-body destroy-on-close>
    <ElForm label-position="top">
      <div class="scene-form__row">
        <ElFormItem label="节奏">
          <ElSelect v-model="form.pace_code" class="scene-form__select">
            <ElOption
              v-for="item in PACE_OPTIONS"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="预计字数">
          <ElInputNumber v-model="form.estimated_words" :min="0" :max="100000" :step="500" />
        </ElFormItem>
      </div>

      <ElFormItem label="出场角色">
        <ElSelect
          v-model="form.character_ulid_list"
          class="scene-form__select"
          multiple
          collapse-tags
          collapse-tags-tooltip
          placeholder="选择本场戏出场的角色"
        >
          <ElOption
            v-for="character in characterOptions"
            :key="character.character_ulid"
            :label="character.character_name"
            :value="character.character_ulid"
          />
        </ElSelect>
      </ElFormItem>

      <ElFormItem label="地点">
        <ElInput v-model="form.location_text" maxlength="128" placeholder="场景发生在哪里" />
      </ElFormItem>

      <ElFormItem label="冲突">
        <ElInput
          v-model="form.conflict_text"
          type="textarea"
          :rows="2"
          maxlength="1024"
          placeholder="本场戏的核心矛盾"
        />
      </ElFormItem>

      <div class="scene-form__row">
        <ElFormItem label="入场状态">
          <ElInput v-model="form.entry_state_text" type="textarea" :rows="2" maxlength="1024" />
        </ElFormItem>
        <ElFormItem label="出场状态">
          <ElInput v-model="form.exit_state_text" type="textarea" :rows="2" maxlength="1024" />
        </ElFormItem>
      </div>

      <ElFormItem label="目标">
        <ElInput v-model="form.goal_text" type="textarea" :rows="2" maxlength="1024" />
      </ElFormItem>

      <ElFormItem label="关键抉择">
        <ElInput v-model="form.key_choice_text" type="textarea" :rows="2" maxlength="1024" />
      </ElFormItem>

      <ElFormItem label="结果">
        <ElInput v-model="form.outcome_text" type="textarea" :rows="2" maxlength="1024" />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .scene-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }
  }
</style>
