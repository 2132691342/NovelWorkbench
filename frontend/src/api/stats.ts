// 数据总览接口：项目数、字数统计与趋势。
import { get } from '@/api/http'
import type { NWStatsOverviewVo } from './contracts'

/** 数据总览：项目数、总字数、今日新增、AI 字数与 30 天趋势 */
export function getStatsOverview(): Promise<NWStatsOverviewVo> {
  return get<NWStatsOverviewVo>('/stats/overview')
}
