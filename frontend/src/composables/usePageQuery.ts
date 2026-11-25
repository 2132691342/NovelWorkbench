// 分页列表状态：统一管理页码、筛选条件、加载态与结果。
import { ref, type Ref } from 'vue'
import type { NWPageDto } from '@/api/contracts'

/** 分页查询请求（后端 PageREQ 约定） */
export interface PageQuery {
  page_num: number
  page_size: number
}

/**
 * 分页列表通用状态：查询参数、页码、总数、加载态与错误。不含表格/表单 UI，各页面自行决定渲染形态。
 */
export const usePageQuery = <Row, Query extends object>(
  fetcher: (query: Query & PageQuery) => Promise<NWPageDto<Row>>,
  initialQuery: Query
) => {
  const items = ref([]) as Ref<Row[]>
  const total = ref(0)
  const pageNum = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const query = ref({ ...initialQuery }) as Ref<Query>

  const load = async () => {
    loading.value = true
    try {
      const page = await fetcher({ ...query.value, page_num: pageNum.value, page_size: pageSize.value })
      items.value = page.items ?? []
      total.value = page.total ?? 0
    } finally {
      loading.value = false
    }
  }

  /** 条件查询：重置到第一页 */
  const search = async () => {
    pageNum.value = 1
    await load()
  }

  const changePage = async (nextPageNum: number) => {
    pageNum.value = nextPageNum
    await load()
  }

  return { items, total, pageNum, pageSize, loading, query, load, search, changePage }
}
