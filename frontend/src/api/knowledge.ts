// 知识库文档接口封装（列表/上传/删除/重索引/检索/切片/原文，字段对齐后端 domain/knowledge.go）
import { post, get } from '@/api/http'
import { readToken } from '@/utils/storage'
import type {
  NWKnowledgeChunkVo,
  NWKnowledgeDocumentVo,
  NWKnowledgeListReqVo,
  NWPageDto,
} from './contracts'

/** 知识库检索请求（字段对齐后端 KnowledgeSearchREQ） */
export interface NWKnowledgeSearchReqVo {
  project_ulid: string
  keyword_text?: string
  query_text?: string
  search_mode?: string
  top_k_count?: number
  doc_ulid_list?: string[]
}

/** 检索追踪（字段对齐后端 KnowledgeSearchTraceVO） */
export interface NWKnowledgeSearchTraceVo {
  query_terms: string[]
  query_expr: string
  candidate_count: number
  reranked: boolean
  rerank_note: string
  selected_ids: string[]
  source_mode: string
}

/** 知识库检索响应（字段对齐后端 KnowledgeSearchVO） */
export interface NWKnowledgeSearchVo {
  result_list: NWKnowledgeChunkVo[]
  trace: NWKnowledgeSearchTraceVo
}

/** 知识库文档分页列表 */
export function listKnowledgeDocuments(
  req: NWKnowledgeListReqVo,
): Promise<NWPageDto<NWKnowledgeDocumentVo>> {
  return post<NWPageDto<NWKnowledgeDocumentVo>>('/knowledge/list', req, { silent_error: true })
}

/** 上传知识库文档（multipart：file + project_ulid + doc_name，可回调上传进度） */
export function uploadKnowledgeDocument(params: {
  file: File
  projectUlid: string
  onUploadProgress?: (percent: number) => void
}): Promise<NWKnowledgeDocumentVo> {
  const formData = new FormData()
  formData.append('file', params.file)
  formData.append('project_ulid', params.projectUlid)
  formData.append('doc_name', params.file.name)
  return post<NWKnowledgeDocumentVo>('/knowledge/upload', formData, {
    timeout: 120000,
    onUploadProgress: (event) => {
      if (!params.onUploadProgress || !event.total) {
        return
      }
      params.onUploadProgress(Math.round((event.loaded / event.total) * 100))
    },
  })
}

/** 删除文档（连带删原文/切片/索引） */
export async function deleteKnowledgeDocument(docUlid: string): Promise<void> {
  await post('/knowledge/delete', { doc_ulid: docUlid })
}

/** 重投索引任务（仅 indexed/failed 可重投） */
export async function reindexKnowledgeDocument(docUlid: string): Promise<void> {
  await post('/knowledge/reindex', { doc_ulid: docUlid })
}

/** 知识库检索（keyword 关键词 / hybrid 词法混合） */
export function searchKnowledge(req: NWKnowledgeSearchReqVo): Promise<NWKnowledgeSearchVo> {
  return post<NWKnowledgeSearchVo>('/knowledge/search', req)
}

/** 文档切片分页（预览/切片列表弹窗用） */
export function listKnowledgeChunks(params: {
  docUlid: string
  pageNum: number
  pageSize: number
}): Promise<NWPageDto<NWKnowledgeChunkVo>> {
  return get<NWPageDto<NWKnowledgeChunkVo>>(
    `/knowledge/chunk/list/${encodeURIComponent(params.docUlid)}` +
      `&page_num=${params.pageNum}&page_size=${params.pageSize}`,
    { silent_error: true },
  )
}

/** 读取文档原文二进制（GET /knowledge/content，docx/pdf 预览用；二进制流不走包络解包） */
export async function fetchKnowledgeContent(docUlid: string): Promise<Blob> {
  const response = await fetch(
    `/api/v1/knowledge/content/${encodeURIComponent(docUlid)}`,
    { headers: { Authorization: `Bearer ${readToken()}` } },
  )
  if (!response.ok) {
    throw new Error(`knowledge content http ${response.status}`)
  }
  return response.blob()
}
