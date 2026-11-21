<!-- 菜单编辑弹窗：维护菜单的父级、路由与权限码。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDialog,
    ElForm,
    ElFormItem,
    ElInput,
    ElInputNumber,
    ElMessage,
    ElOption,
    ElSelect,
    ElSwitch,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { saveMenu, type NWMenuSaveReqVo } from '@/api/menu'
  import type { NWMenuVo } from '@/api/contracts'

  const props = defineProps<{ menuList: NWMenuVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWMenuVo | null>(null)

  const form = reactive({
    parent_ulid: '',
    menu_code: '',
    menu_name: '',
    menu_type_code: 'page',
    scope_code: 'global',
    route_path: '',
    icon_code: '',
    perm_code: '',
    visible_flag: true,
    enabled_flag: true,
    sort_order: 0,
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑菜单' : '新建菜单'))

  const typeOptions = [
    { value: 'group', label: '分组' },
    { value: 'page', label: '页面' },
    { value: 'action', label: '动作' }
  ]

  const scopeOptions = [
    { value: 'global', label: '全局' },
    { value: 'project', label: '项目内' }
  ]

  /** 可选上级：排除自身与其后代，避免成环 */
  const parentOptions = computed(() => {
    const selfUlid = editing.value?.menu_ulid
    if (!selfUlid) return props.menuList
    const childSet = new Set<string>()
    const collect = (parentUlid: string) => {
      props.menuList.forEach((item) => {
        if (item.parent_ulid === parentUlid && !childSet.has(item.menu_ulid)) {
          childSet.add(item.menu_ulid)
          collect(item.menu_ulid)
        }
      })
    }
    collect(selfUlid)
    return props.menuList.filter((item) => item.menu_ulid !== selfUlid && !childSet.has(item.menu_ulid))
  })

  const rules = computed<FormRules>(() => ({
    menu_code: [
      { required: true, message: '请输入菜单编码', trigger: 'blur' },
      { min: 2, max: 64, message: '长度 2-64 个字符', trigger: 'blur' }
    ],
    menu_name: [{ required: true, message: '请输入菜单名称', trigger: 'blur' }],
    menu_type_code: [{ required: true, message: '请选择菜单类型', trigger: 'change' }]
  }))

  /** 打开弹窗：传菜单为编辑，不传为新建 */
  const open = (menu?: NWMenuVo) => {
    editing.value = menu ?? null
    form.parent_ulid = menu?.parent_ulid ?? ''
    form.menu_code = menu?.menu_code ?? ''
    form.menu_name = menu?.menu_name ?? ''
    form.menu_type_code = menu?.menu_type_code ?? 'page'
    form.scope_code = menu?.scope_code || 'global'
    form.route_path = menu?.route_path ?? ''
    form.icon_code = menu?.icon_code ?? ''
    form.perm_code = menu?.perm_code ?? ''
    form.visible_flag = menu?.visible_flag ?? true
    form.enabled_flag = menu?.enabled_flag ?? true
    form.sort_order = menu?.sort_order ?? 0
    form.remark_text = menu?.remark_text ?? ''
    visible.value = true
    formRef.value?.clearValidate()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      const payload: NWMenuSaveReqVo = {
        menu_code: form.menu_code,
        menu_name: form.menu_name,
        menu_type_code: form.menu_type_code,
        scope_code: form.scope_code,
        route_path: form.route_path,
        icon_code: form.icon_code,
        perm_code: form.perm_code,
        visible_flag: form.visible_flag,
        enabled_flag: form.enabled_flag,
        sort_order: form.sort_order,
        remark_text: form.remark_text
      }
      if (form.parent_ulid) payload.parent_ulid = form.parent_ulid
      if (editing.value) payload.menu_ulid = editing.value.menu_ulid
      await saveMenu(payload)
      ElMessage.success(editing.value ? '菜单已更新' : '菜单已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="600px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="menu-form__row">
        <ElFormItem label="上级菜单">
          <ElSelect v-model="form.parent_ulid" clearable placeholder="不选为根级" class="menu-form__field">
            <ElOption
              v-for="item in parentOptions"
              :key="item.menu_ulid"
              :label="`${item.menu_name}（${item.menu_code}）`"
              :value="item.menu_ulid"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="菜单类型" prop="menu_type_code">
          <ElSelect v-model="form.menu_type_code" class="menu-form__field">
            <ElOption
              v-for="item in typeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
      </div>
      <div class="menu-form__row">
        <ElFormItem label="菜单编码" prop="menu_code">
          <ElInput v-model="form.menu_code" maxlength="64" placeholder="例如 admin_users" />
        </ElFormItem>
        <ElFormItem label="菜单名称" prop="menu_name">
          <ElInput v-model="form.menu_name" maxlength="64" placeholder="例如 用户管理" />
        </ElFormItem>
      </div>
      <div class="menu-form__row">
        <ElFormItem label="作用域">
          <ElSelect v-model="form.scope_code" class="menu-form__field">
            <ElOption
              v-for="item in scopeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="排序号">
          <ElInputNumber
            v-model="form.sort_order"
            :min="0"
            :max="9999"
            controls-position="right"
            class="menu-form__field"
          />
        </ElFormItem>
      </div>
      <ElFormItem label="路由路径">
        <ElInput v-model="form.route_path" maxlength="128" placeholder="例如 /admin/users" />
      </ElFormItem>
      <div class="menu-form__row">
        <ElFormItem label="图标编码">
          <ElInput v-model="form.icon_code" maxlength="32" placeholder="例如 mdi:account-group" />
        </ElFormItem>
        <ElFormItem label="权限码">
          <ElInput v-model="form.perm_code" maxlength="64" placeholder="留空表示登录即可见" />
        </ElFormItem>
      </div>
      <div class="menu-form__row">
        <ElFormItem label="导航展示">
          <ElSwitch v-model="form.visible_flag" />
        </ElFormItem>
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled_flag" />
        </ElFormItem>
      </div>
      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="2" maxlength="256" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .menu-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__field {
      width: 100%;
    }
  }
</style>
