// 斜杠命令注册表的回归：解析口径必须与后端一致，否则同一句 /polish 在前后端会走向不同分支。
// 解析错了的后果不是报错而是静默走错链路：本地动作被当成提示词发给模型，或正文被当成命令吞掉。
import { describe, expect, it } from 'vitest'
import {
  COMPOSER_ACTION_LIST,
  actionCodeSet,
  filterActions,
  matchActionByCode,
  parseLeadingSlashToken
} from './composerCommands'

describe('parseLeadingSlashToken', () => {
  it('解析首 token 命令并剥离正文', () => {
    expect(parseLeadingSlashToken('/polish 加强画面感')).toEqual({
      actionCode: 'polish',
      remainderText: '加强画面感'
    })
    expect(parseLeadingSlashToken('  /clear  ')).toEqual({ actionCode: 'clear', remainderText: '' })
  })

  it('非首 token、非命令字符与粘连正文都不算命令', () => {
    expect(parseLeadingSlashToken('续写 /continue')).toBeNull()
    expect(parseLeadingSlashToken('/ 加强画面感')).toBeNull()
    expect(parseLeadingSlashToken('/Polish 大写')).toBeNull()
    expect(parseLeadingSlashToken('/po!lish')).toBeNull()
    expect(parseLeadingSlashToken('')).toBeNull()
  })

  it('超长编码不匹配，避免把整段路径当命令', () => {
    expect(parseLeadingSlashToken(`/${'a'.repeat(33)} 正文`)).toBeNull()
    expect(parseLeadingSlashToken(`/${'a'.repeat(32)} 正文`)?.actionCode).toBe('a'.repeat(32))
  })
})

describe('action registry', () => {
  it('动作编码唯一且都能按编码取回', () => {
    const codeSet = actionCodeSet()
    expect(codeSet.size).toBe(COMPOSER_ACTION_LIST.length)
    for (const action of COMPOSER_ACTION_LIST) {
      expect(matchActionByCode(action.action_code)?.action_code).toBe(action.action_code)
    }
    expect(matchActionByCode('not_exist')).toBeNull()
  })

  it('动作编码不与内置提示词编码冲突（保留命名空间）', () => {
    const builtinPromptCodes = ['continue', 'polish', 'deai', 'review', 'hitscan', 'summary']
    for (const promptCode of builtinPromptCodes) {
      expect(actionCodeSet().has(promptCode)).toBe(false)
    }
  })

  it('关键词过滤命中编码、名称与说明', () => {
    expect(filterActions('').length).toBe(COMPOSER_ACTION_LIST.length)
    expect(filterActions('clear').map((action) => action.action_code)).toContain('clear')
    expect(filterActions('体检').map((action) => action.action_code)).toContain('check')
    expect(filterActions('收起目录树').map((action) => action.action_code)).toContain('focus')
    expect(filterActions('不存在的东西')).toHaveLength(0)
  })

  it('已移除历史条数与压缩动作：历史改为全量注入', () => {
    const codeList = COMPOSER_ACTION_LIST.map((action) => action.action_code)
    expect(codeList).not.toContain('compact')
    expect(codeList).not.toContain('history')
  })
})
