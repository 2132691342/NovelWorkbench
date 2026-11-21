<!-- 角色卡编辑抽屉：维护角色基础信息与画像。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    ElOption,
    ElSelect,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { saveCharacter } from '@/api/character'
  import type { NWCharacterVo } from '@/api/character'

  const props = defineProps<{ projectUlid: string }>()
  const emit = defineEmits<{ saved: [] }>()

  /** 角色类型选项（与后端 role_type_code 枚举一致） */
  const roleTypeOptions = [
    { value: 'protagonist', label: '主角' },
    { value: 'antagonist', label: '反派' },
    { value: 'supporting', label: '配角' },
    { value: 'minor', label: '龙套' }
  ]

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editingUlid = ref('')

  const form = reactive({
    character_name: '',
    alias_text: '',
    role_type_code: 'supporting',
    personality_text: '',
    background_text: '',
    appearance_text: '',
    goal_text: ''
  })

  const title = computed(() => (editingUlid.value ? '编辑角色' : '新建角色'))

  const rules: FormRules = {
    character_name: [{ required: true, message: '请输入角色姓名', trigger: 'blur' }]
  }

  /** 打开抽屉：传角色为编辑，不传为新建 */
  const open = (character?: NWCharacterVo) => {
    editingUlid.value = character?.character_ulid ?? ''
    form.character_name = character?.character_name ?? ''
    form.alias_text = character?.alias_text ?? ''
    form.role_type_code = character?.role_type_code || 'supporting'
    form.personality_text = character?.personality_text ?? ''
    form.background_text = character?.background_text ?? ''
    form.appearance_text = character?.appearance_text ?? ''
    form.goal_text = character?.goal_text ?? ''
    formRef.value?.clearValidate()
    visible.value = true
  }

  const submit = async () => {
    if (!props.projectUlid) {
      ElMessage.warning('请先选择项目')
      return
    }
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    saving.value = true
    try {
      await saveCharacter({
        character_ulid: editingUlid.value || undefined,
        project_ulid: props.projectUlid,
        character_name: form.character_name.trim(),
        alias_text: form.alias_text.trim(),
        role_type_code: form.role_type_code,
        personality_text: form.personality_text,
        background_text: form.background_text,
        appearance_text: form.appearance_text,
        goal_text: form.goal_text
      })
      ElMessage.success(editingUlid.value ? '角色已更新' : '角色已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="480px" append-to-body>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" class="character-form">
      <ElFormItem label="姓名" prop="character_name">
        <ElInput v-model="form.character_name" maxlength="64" show-word-limit placeholder="角色姓名" />
      </ElFormItem>
      <ElFormItem label="别名">
        <ElInput v-model="form.alias_text" maxlength="128" placeholder="外号、尊称、化名，多个用顿号分隔" />
      </ElFormItem>
      <ElFormItem label="角色类型">
        <ElSelect v-model="form.role_type_code" class="character-form__select">
          <ElOption
            v-for="item in roleTypeOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="性格">
        <ElInput
          v-model="form.personality_text"
          type="textarea"
          :rows="2"
          maxlength="2048"
          placeholder="性格底色、行为习惯、口癖"
        />
      </ElFormItem>
      <ElFormItem label="外貌">
        <ElInput
          v-model="form.appearance_text"
          type="textarea"
          :rows="2"
          maxlength="2048"
          placeholder="外貌特征与标志性装扮"
        />
      </ElFormItem>
      <ElFormItem label="背景经历">
        <ElInput
          v-model="form.background_text"
          type="textarea"
          :rows="3"
          maxlength="4096"
          placeholder="出身、经历与关键转折"
        />
      </ElFormItem>
      <ElFormItem label="目标动机">
        <ElInput
          v-model="form.goal_text"
          type="textarea"
          :rows="2"
          maxlength="2048"
          placeholder="当前想要什么、为何而行动"
        />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton size="small" @click="visible = false">取消</ElButton>
      <ElButton type="primary" size="small" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .character-form {
    &__select {
      width: 100%;
    }
  }
</style>
