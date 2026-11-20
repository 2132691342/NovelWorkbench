// 写作台 AI 面板的快捷指令清单（仅描述结构，文案由组件按 i18n 键渲染）。
export interface NWQuickPrompt {
  /** 指令编码（前端标识，非技能编码） */
  prompt_code: string
  /** 标题 i18n 键 */
  title_key: string
  /** 说明 i18n 键 */
  desc_key: string
  /** 填入输入框的提示词 i18n 键 */
  text_key: string
  /** 关联技能编码（空表示自由对话） */
  skill_code: string
}

/** 面板空态展示的快捷指令 */
export const NW_QUICK_PROMPT_LIST: NWQuickPrompt[] = [
  {
    prompt_code: 'continue',
    title_key: 'workspace.ai.quickContinueTitle',
    desc_key: 'workspace.ai.quickContinueDesc',
    text_key: 'workspace.ai.quickContinuePrompt',
    skill_code: 'continue',
  },
  {
    prompt_code: 'consistency',
    title_key: 'workspace.ai.quickCheckTitle',
    desc_key: 'workspace.ai.quickCheckDesc',
    text_key: 'workspace.ai.quickCheckPrompt',
    skill_code: '',
  },
  {
    prompt_code: 'directions',
    title_key: 'workspace.ai.quickBranchTitle',
    desc_key: 'workspace.ai.quickBranchDesc',
    text_key: 'workspace.ai.quickBranchPrompt',
    skill_code: '',
  },
]
