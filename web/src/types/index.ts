/**
 * 全局类型定义 —— 严格对应 Go 后端数据模型与 API 契约。
 * 时间字段均为 RFC3339 字符串；标注为 JSON 字符串的字段后端返回 string，需 safeParseJSON。
 */

/* ============================== 通用信封 ============================== */
export interface ApiEnvelope<T = unknown> {
  success: boolean
  data: T
  error: string | null
}

/* ============================== 认证 ============================== */
export interface LoginRequest {
  username: string
  password: string
}

export interface UserInfo {
  id: string
  username: string
  name: string
  role: string
}

export interface LoginResponse {
  token: string
  user: UserInfo
}

export interface RefreshRequest {
  token: string
}

export interface AuthState {
  loggedIn: boolean
  username: string
  name: string
  role: string
  token: string
}

/* ============================== 设备 ============================== */
export interface DeviceInfo {
  serial: string
  model: string
  manufacturer: string
  androidVer: string
  sdkVersion: string
  battery: string
  resolution: string
  state: string
}

/* ============================== 脚本 ============================== */
export type ScriptLanguage = 'python' | 'shell' | 'javascript'

export interface Script {
  id: string
  name: string
  description: string
  language: ScriptLanguage
  code: string
  createdAt: string
  updatedAt: string
}

/* ============================== 执行任务 ============================== */
export type ExecutionStatus = 'pending' | 'running' | 'success' | 'failed' | 'cancelled'

export interface Execution {
  id: string
  scriptId: string
  deviceSerial: string
  taskName: string
  status: ExecutionStatus
  logs: string // JSON 字符串数组
  screenshots: string // JSON 字符串数组
  duration: number
  startedAt: string
  finishedAt: string
  createdAt: string
}

export interface CreateExecutionRequest {
  scriptId: string
  deviceSerial?: string
  taskName?: string
}

/* ============================== 缺陷 ============================== */
export type BugSeverity = 'critical' | 'major' | 'minor' | 'trivial'
export type BugStatus = 'open' | 'in_progress' | 'resolved' | 'closed' | 'reopened'

export interface Bug {
  id: string
  title: string
  severity: BugSeverity
  priority: 'P0' | 'P1' | 'P2' | 'P3'
  status: BugStatus
  assignee: string
  reporter: string
  module: string
  env: string
  description: string
  steps: string
  expected: string
  actual: string
  tags: string // JSON 字符串数组
  relatedCaseId: string
  externalId: string
  externalUrl: string
  createdAt: string
  updatedAt: string
}

export type BugStats = Record<string, number>

/* ============================== 测试用例 ============================== */
export type Priority = 'P0' | 'P1' | 'P2' | 'P3'

export interface CaseStep {
  action: string
  expected: string
  actual?: string
  status?: string
  screenshot?: string
}

export interface TestCase {
  id: string
  name: string
  moduleId: string
  priority: Priority
  type: string
  precondition: string
  steps: string // JSON 字符串数组
  assignee: string
  status: string
  tags: string // JSON 字符串数组
  scriptId?: string // 关联自动化脚本（计划自动执行）
  createdAt: string
  updatedAt: string
}

export interface CaseModule {
  id: string
  name: string
  parentId: string | null
  sortOrder: number
  createdAt: string
}

/* ============================== 用例执行记录 ============================== */
export type ExecResult = 'pending' | 'passed' | 'failed' | 'skipped' | 'blocked'

export interface CaseExecution {
  id: string
  caseId: string
  caseName: string
  executor: string
  result: ExecResult
  steps: string // JSON 字符串数组
  duration: number
  remark: string
  executedAt: string
  planId?: string
  executionId?: string
}

export type CaseExecStats = Record<string, number>

/* ============================== 测试计划 ============================== */
export type TestPlanStatus = 'draft' | 'in_progress' | 'completed' | 'cancelled'

export interface TestPlan {
  id: string
  name: string
  description: string
  caseIds: string // JSON 字符串数组
  status: TestPlanStatus
  startDate: string
  endDate: string
  createdAt: string
  updatedAt: string
}

export interface PlanExecution {
  id: string
  planId: string
  planName: string
  status: string
  result: string // JSON
  casesTotal: number
  casesPassed: number
  casesFailed: number
  duration: number
  startedAt: string
  finishedAt: string
  createdAt: string
  // 登记执行结果新增字段
  executedBy: string
  casesDetail: string // JSON: [{caseId,caseName,result,remark}]
}

/** 计划执行中的逐用例明细项 */
export interface PlanCaseDetail {
  caseId: string
  caseName: string
  result: ExecResult
  remark: string
}

export interface AutoTaskExecution {
  id: string
  taskName: string
  status: string
  result: string
  duration: number
  createdAt: string
  [key: string]: unknown
}

/* ============================== 接口定义 ============================== */
export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'HEAD' | 'OPTIONS' | string

export interface APIDefinition {
  id: string
  name: string
  method: HttpMethod
  url: string
  tags: string // JSON 字符串数组
  moduleId: string
  headers: string // JSON
  body: string
  createdAt: string
  updatedAt: string
}

export interface APIDefModule {
  id: string
  name: string
  parentId: string
  sortOrder: number
  createdAt: string
}

/* ============================== 接口测试 ============================== */
export interface APIRequest {
  id: string
  name: string
  method: HttpMethod
  url: string
  headers: string // JSON
  params: string // JSON
  body: string
  description: string
  tags: string // JSON
  folderId: string
  createdAt: string
  updatedAt: string
}

export interface APIFolder {
  id: string
  name: string
  parentId: string
  sortOrder: number
  createdAt: string
}

export interface APIHistory {
  id: string
  requestId: string
  method: HttpMethod
  url: string
  response: string
  statusCode: number
  duration: number
  createdAt: string
}

export interface SendApiRequest {
  method: HttpMethod
  url: string
  headers?: Record<string, string>
  params?: Record<string, string>
  body?: string
}

export interface ApiTestResult {
  status: number
  statusText?: string
  headers?: Record<string, string>
  body?: string
  duration?: number
  error?: string
}

/* ============================== gRPC 代理 ============================== */
export interface ProxyStatus {
  running: boolean
  paused: boolean
  port: number
  target: string
  pendingCount: number
  logDir: string
  wsClientCount: number
}

export interface ProxyExecution {
  id: string
  method: string
  target: string
  timestamp: string
  elapsedMs: number
}

export interface ProxySendRequest {
  method: string
  request: string
  headers?: Record<string, string>
  target?: string
  timeout?: number
}

export interface ProxyReplayRequest {
  target: string
  method: string
  rawFrameBase64: string
}

/* ============================== Proto ============================== */
export interface ProtoService {
  name: string
  methods: { name: string; inputType: string; outputType: string }[]
}

/* ============================== 设置 ============================== */
export type SettingsMap = Record<string, unknown>

/* ============================== 迁移 ============================== */
export interface MigrationStatus {
  scripts?: number
  test_cases?: number
  bugs?: number
  api_definitions?: number
  settings?: number
  [key: string]: unknown
}

export interface MigrationImportPayload {
  scripts?: unknown[]
  test_cases?: unknown[]
  bugs?: unknown[]
  api_definitions?: unknown[]
  settings?: SettingsMap
}

/* ============================== 日志 ============================== */
export interface LogEntry {
  time: string
  level: string
  message: string
  [key: string]: unknown
}

/* ============================== SDK ============================== */
export interface SdkItem {
  engine: string
  engineLabel?: string
  file: string
  size?: number
  reportToken?: string
  [key: string]: unknown
}

/* ============================== SDK 上报 ============================== */
export interface QaReport {
  id: string
  event: string
  name: string
  result: string
  message: string
  tags: string
  token: string
  source: string
  timestamp: number
  createdAt: string
  seq: number
  method: string
  headers: string
  reqBody: string
  respBody: string
  errMsg: string
  elapsedMs: number
  ts: string
}

/* ============================== 表格 / 思维导图用例 ============================== */
export interface TableCase {
  id: string
  name: string
  moduleId: string
  priority: Priority
  type: string
  precondition: string
  steps: string
  assignee: string
  status: string
  tags: string
  createdAt: string
  updatedAt: string
}

export interface TableModule {
  id: string
  name: string
  parentId: string | null
  sortOrder: number
  createdAt: string
}

export interface XmindCase {
  id: string
  name: string
  moduleId: string
  parentId: string // 父节点 ID，空字符串表示中心主题（根）
  priority: Priority
  type: string
  precondition: string
  steps: string
  assignee: string
  status: string
  tags: string
  createdAt: string
  updatedAt: string
}

/** 自由电子表格（纯文本网格，cells 为二维字符串数组） */
export interface Spreadsheet {
  id: string
  name: string
  cells: string[][] // 二维字符串数组
  createdAt: string
  updatedAt: string
}

export interface XmindModule {
  id: string
  name: string
  parentId: string | null
  sortOrder: number
  createdAt: string
}

/* ============================== 枚举辅助 ============================== */
export const PRIORITY_OPTIONS: Priority[] = ['P0', 'P1', 'P2', 'P3']
export const BUG_SEVERITY_OPTIONS: BugSeverity[] = ['critical', 'major', 'minor', 'trivial']
export const BUG_STATUS_OPTIONS: BugStatus[] = [
  'open',
  'in_progress',
  'resolved',
  'closed',
  'reopened',
]
export const EXEC_RESULT_OPTIONS: ExecResult[] = [
  'pending',
  'passed',
  'failed',
  'skipped',
  'blocked',
]
export const HTTP_METHOD_OPTIONS: HttpMethod[] = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']

export const SEVERITY_LABEL: Record<BugSeverity, string> = {
  critical: '致命',
  major: '严重',
  minor: '一般',
  trivial: '轻微',
}
export const BUG_STATUS_LABEL: Record<BugStatus, string> = {
  open: '待处理',
  in_progress: '处理中',
  resolved: '已解决',
  closed: '已关闭',
  reopened: '重新打开',
}
export const EXEC_RESULT_LABEL: Record<ExecResult, string> = {
  pending: '待执行',
  passed: '通过',
  failed: '失败',
  skipped: '跳过',
  blocked: '阻塞',
}
