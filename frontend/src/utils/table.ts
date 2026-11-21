/**
 * ElTable 的 scoped slot 把 row 推断成 DefaultRow（空对象），无法从 :data 反推行类型。
 * 全站统一用本函数在模板里取回真实行类型，避免每处各写一次 `row as XxxVo`。
 * 调用形如 `asRow<NWUserAdminVo>(row)`；行数据由 ElTable 原样透传，类型由调用方声明。
 */
export const asRow = <Row>(row: unknown): Row => row as Row
