/**
 * ElTable 的 scoped slot 把 row 推断成 DefaultRow（空对象），无法从 :data 反推行类型。
 * 全站统一用本函数取回真实行类型，调用形如 `asRow<NWUserAdminVo>(row)`。
 */
export const asRow = <Row>(row: unknown): Row => row as Row
