// Package repository 集中管理所有资源域的数据访问（SQL 层）。
//
// 背景：此前 handlers 包内散落 100+ 处 database.DB 直连 SQL，同构 CRUD 多处复制；
// 本包把 SQL 收敛到按领域划分的文件中，handlers 只保留
// 「请求解析 → 调用 repository → 响应/错误映射」的 HTTP 职责。
//
// 约定（迁移自 handlers 的 SQL 必须遵守）：
//   - 函数直接使用 database.DB 全局连接：单实例 SQLite 应用，连接由 database.Init
//     唯一建立；连接注入留待真正需要多存储后端时再引入。
//   - SQL 语句从原 handler 原样迁入：列顺序、WHERE、ORDER、LIMIT 一律不改，避免行为漂移。
//   - 返回原始 error（含 sql.ErrNoRows），HTTP 状态码映射仍由 handlers 负责。
//   - 新增领域时新建独立文件（如 repository/xxx.go），不要把多个领域混在一个文件。
package repository
