// 模型页共享的本地类型：连通性测试结果的展示形态。

/** 单个对象的最近一次测试结果（延迟与结论） */
export interface ModelTestResult {
  status_code: string
  latency_ms: number
  message_text: string
}
