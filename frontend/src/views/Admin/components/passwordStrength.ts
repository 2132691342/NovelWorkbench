// 密码强度判定：新建用户与改密共用同一套规则（≥8 位、含字母与数字），避免各页标准不一致。
export interface PasswordStrength {
  /** 强度档位：0 空 / 1 弱 / 2 中 / 3 强 */
  score: number
  /** 档位对应的配色（弱/中/强） */
  tone: 'weak' | 'medium' | 'strong'
  /** 面向用户的提示文案 */
  hint: string
  /** 是否满足最低要求（后端校验与前端表单共用） */
  passed: boolean
}

/** 判定密码强度 */
export const checkPasswordStrength = (passwordText: string): PasswordStrength => {
  if (!passwordText) {
    return { score: 0, tone: 'weak', hint: '请输入密码', passed: false }
  }
  const hasLetter = /[A-Za-z]/.test(passwordText)
  const hasDigit = /\d/.test(passwordText)
  const hasSymbol = /[^A-Za-z0-9]/.test(passwordText)
  const longEnough = passwordText.length >= 8
  const passed = longEnough && hasLetter && hasDigit

  if (!passed) {
    const missing: string[] = []
    if (!longEnough) missing.push('至少 8 位')
    if (!hasLetter) missing.push('包含字母')
    if (!hasDigit) missing.push('包含数字')
    return { score: 1, tone: 'weak', hint: `还差：${missing.join('、')}`, passed: false }
  }
  if (passwordText.length >= 12 && hasSymbol) {
    return { score: 3, tone: 'strong', hint: '强度：强', passed: true }
  }
  return { score: 2, tone: 'medium', hint: '强度：中（加长或加符号可更强）', passed: true }
}
