// 斜杠命令（技能）接口封装
import { get } from '@/api/http'
import type { NWSlashCommandItemVo } from './contracts'

/** 斜杠命令列表（enabled_only=true 仅返回启用命令，供聊天技能自动补全使用） */
export function listSlashCommands(enabledOnly = true): Promise<NWSlashCommandItemVo[]> {
  return get<NWSlashCommandItemVo[]>(`/slash/list?enabled_only=${String(enabledOnly)}`, {
    silent_error: true,
  })
}
