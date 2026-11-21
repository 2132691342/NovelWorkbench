// 表单校验与提交流程的回归测试：规则校验 → 提交拦截 → 服务端错误回填 → 重置。
// 登录表单与所有抽屉表单共用这一套表单逻辑，校验失效会直接放行非法输入，因此整体守住。

import { describe, expect, it } from 'vitest'
import { required, useForm } from '../src/useForm'

type Values = {
  username: string
  password: string
  confirmPassword: string
}

/** 构造一份受测表单。 */
function createForm() {
  return useForm<Values>({
    initialValues: { username: '', password: '', confirmPassword: '' },
    rules: {
      username: required('username is required'),
      password: required('password is required'),
      confirmPassword: (value, values) =>
        value === values.password ? undefined : 'passwords do not match'
    }
  })
}

describe('useForm 表单链路', () => {
  it('校验失败不提交且记录错误，校验通过才提交，服务端错误可回填，重置回到初值', async () => {
    const form = createForm()
    expect(form.state.dirty.value).toBe(false)

    // 校验失败：提交被拦截，错误按字段记录。
    let submitted = false
    const invalidResult = await form.actions.submit(() => {
      submitted = true
    })
    expect(invalidResult).toBeUndefined()
    expect(submitted).toBe(false)
    expect(form.state.errors.value.username).toEqual(['username is required'])
    expect(form.state.touchedFields.value).toEqual(['username', 'password', 'confirmPassword'])

    // 改动后进入脏状态；联动规则（两次密码一致）单独校验。
    form.actions.setValues({ username: 'admin', password: 'secret', confirmPassword: 'different' })
    expect(form.state.dirty.value).toBe(true)
    expect(await form.actions.validateField('confirmPassword')).toBe(false)
    expect(form.state.errors.value.confirmPassword).toEqual(['passwords do not match'])

    // 服务端返回的错误可以回填到指定字段并单独清除。
    form.actions.setValue('confirmPassword', 'secret')
    form.actions.setErrors({ username: 'server rejected username' })
    expect(form.state.errors.value.username).toEqual(['server rejected username'])
    form.actions.clearErrors('username')

    // 校验通过才提交，提交成功后错误清空。
    const result = await form.actions.submit((values) => `${values.username}:ok`)
    expect(result).toBe('admin:ok')
    expect(form.state.submitCount.value).toBe(2)
    expect(form.state.hasErrors.value).toBe(false)

    // 重置回到给定初值，脏状态与错误同时清空。
    form.actions.reset({ username: 'remembered', password: 'saved', confirmPassword: 'saved' })
    expect(form.state.dirty.value).toBe(false)
    expect(form.state.values.value.username).toBe('remembered')
    expect(form.state.errors.value).toEqual({})
    expect(form.state.touchedFields.value).toEqual([])
  })
})
