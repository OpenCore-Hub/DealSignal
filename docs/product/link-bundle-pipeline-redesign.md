# 分享链接包（Link Bundle）— 流水线重构 PRD

> **版本**: v1.0  
> **作者**: PM  
> **日期**: 2026-07-04  
> **状态**: 设计评审

---

## 1. 背景与目标

### 1.1 现状

当前 `/links/new` 页面（`SmartLinkCreator`）采用**单页布局**（左侧文档选择 + 安全配置，右侧评分 + 预览），核心局限：

- **仅支持单文档**: `selectedDocumentId` 是单个 string，`DocumentSelector` 是单选 `<Select>`，API payload 只有 `document_id`。
- **信息密度过高**: 文档选择、4个安全预设、6个安全选项、过期时间、最大访问次数、联系人选择、评分、预览全部挤在一个页面，决策负担大。
- **无法构建"资料包"**: 投行/FA/销售实际场景需要一次分享多份文档（如：BP + 财务模型 + 尽调清单），当前必须逐个创建链接。
- **创建后不可编辑**: 链接已发出，发现漏了文档或安全配置有误，只能重新创建，旧链接作废，接收方体验差。

### 1.2 目标

将分享创建重构为 **3 步流水线**，支持**多文档选择**生成**链接包（Link Bundle）**，且**创建后可反复编辑、实时生效**：

| 步骤 | 页面 | 核心任务 |
|:---:|:---|:---|
| 1 | **选文档** | 浏览/搜索/多选文档，构建文档清单 |
| 2 | **设安全** | 安全预设一键应用 + 精细调优 + 实时评分 |
| 3 | **完成** | 清单复核 → 生成/更新 → 复制/分享 |

### 1.3 用户价值

- **效率**: 一次创建多文档链接包，替代逐文档重复创建。
- **降低认知负荷**: 每步聚焦一个决策维度（选什么 → 怎么保护 → 确认发布）。
- **持续迭代**: 链接已发出后仍可补充文档、调整安全策略，收件人刷新即生效。
- **信心**: 进度条 + 每步的即时反馈让用户知道走到了哪里。

---

## 2. Step 1 — 选文档（Select Documents）

### 2.1 页面布局

```
┌──────────────────────────────────────────────────────┐
│  ← 返回链接列表                                       │
│                                                      │
│  创建链接包                                          │
│  选择要分享的文档，组成一个链接包                      │
│                                                      │
│  ┌──────────────────────────────────────────────┐    │
│  │  🔍 搜索文档...                              │    │
│  ├──────────────────────────────────────────────┤    │
│  │  ☑ 商业计划书 v4.2                    PDF   │    │
│  │  ☐ 财务模型 2026                      XLSX  │    │
│  │  ☑ 尽调清单                          DOCX  │    │
│  │  ☐ 市场分析报告                      PDF   │    │
│  │  ☐ 团队介绍                          PDF   │    │
│  │  ...                                         │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  已选 2 个文档                                       │
│  ┌──────────────────────────────────────────────┐    │
│  │ 📄 商业计划书 v4.2                    ✕      │    │
│  │ 📄 尽调清单                          ✕      │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  可拖拽调整文档在链接包中的展示顺序                     │
│                                                      │
│  ┌──────────────────────────────────────────────┐    │
│  │            ○ 选文档  —  ○ 设安全  —  ○ 完成  │    │
│  │  ●                                           │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│                              [ 取消 ]  [ 下一步 → ]   │
└──────────────────────────────────────────────────────┘
```

### 2.2 交互规则

| 规则 | 说明 |
|:---|:---|
| **搜索** | 实时过滤，支持文档标题和文件名模糊匹配 |
| **多选** | Checkbox 多选（参考 `DocumentPicker` 的 Checkbox 模式） |
| **已选清单** | 底部固定区域展示已选文档标签，支持点击 ✕ 移除 |
| **拖拽排序** | 已选文档可拖拽调整顺序（`@dnd-kit` 或原生实现），决定访客看到的文档展示顺序 |
| **最少1篇** | 未选择任何文档时「下一步」按钮 disabled |
| **进入默认态** | URL 参数 `?documentId=xxx` 自动预选单文档（兼容旧入口） |
| **返回不丢失** | 步骤间状态保留，用户返回 Step 1 时已选项不丢失 |

### 2.3 关键状态

```ts
interface BundleDocumentsState {
  selectedDocuments: Document[];   // 有序数组，按用户排列的顺序
  searchQuery: string;
}
```

### 2.4 与现有组件的关系

- **废弃**: `DocumentSelector.tsx`（单选下拉 → 不再适合多选场景）
- **新增**: `BundleDocumentPicker.tsx` — 基于 `deal-rooms/DocumentPicker.tsx` 模式改造：搜索框 + Checkbox 列表 + 已选标签区 + 拖拽排序
- **复用**: `Skeleton` loading 态、空状态提示模式

---

## 3. Step 2 — 设安全（Security）

### 3.1 页面布局

```
┌──────────────────────────────────────────────────────┐
│  ← 返回上一步                                         │
│                                                      │
│  安全设置                                            │
│  为此链接包选择安全保护级别                            │
│                                                      │
│  ┌───────────────────┬──────────────────────────┐    │
│  │  安全预设          │   安全强度    接收方摩擦   │    │
│  │                   │   ████████░░  8/10        │    │
│  │  🌐 公开分发       │   ████░░░░░░  4/10        │    │
│  │  无验证门槛         │                          │    │
│  │                   │   当前预设：标准尽调        │    │
│  │  🔒 标准尽调  ✓    │   适合融资尽调、项目介绍等  │    │
│  │  邮箱+白名单+水印   │   需要确认访问者身份但无需  │    │
│  │                   │   极端保密的场景。          │    │
│  │  🛡 机密数据室      │                          │    │
│  │  全部门控+NDA      │   开启的安全特性：         │    │
│  │                   │   ✓ 邮箱验证码             │    │
│  │  👥 协作评审       │   ✓ 白名单                │    │
│  │  邮箱+下载+水印     │   ✓ 动态水印              │    │
│  │                   │   ✓ 禁止下载               │    │
│  └───────────────────┴──────────────────────────┘    │
│                                                      │
│  精细安全选项  [展开/收起]                             │
│  ┌──────────────────────────────────────────────┐    │
│  │ ☑ 邮箱验证码    [选择联系人 ▾]               │    │
│  │ ☑ 白名单邮箱/域名  [user@fund.com, @fund.com]│    │
│  │ ☐ 访问密码      [············]              │    │
│  │ ☐ NDA 签署                                    │    │
│  │ ☐ 允许下载                                    │    │
│  │ ☑ 动态水印                                    │    │
│  │                                              │    │
│  │ 有效期  [30天 ▾]   最大访问  [无限制 ▾]       │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌──────────────────────────────────────────────┐    │
│  │            ● 选文档  —  ● 设安全  —  ○ 完成  │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│                        [ 上一步 ]  [ 下一步：预览 → ] │
└──────────────────────────────────────────────────────┘
```

### 3.2 改造要点

进入 Step 2 时，可将大量现有代码**直接复用**，核心改动是**布局适配**：

| 现有组件 | 改造方式 |
|:---|:---|
| `PermissionPanel` | **原样复用**。4个安全预设卡片 + 选中态详情卡片 |
| `SecurityOptions` | **原样复用**。6个 Checkbox + 过期/最大访问。折叠在"精细安全选项"区域，默认展开（保持可见性） |
| `ScoreDisplay` | **原样复用**。安全强度 + 摩擦评分双进度条。位置：预设卡片的右侧面板 |
| `ContactSelector` | **原样复用** |
| `levelConfig.ts` | **零改动**。预设定义、评分算法、跨选项约束全部保持不变 |
| `types.ts` (PermissionConfig) | **零改动**。`PermissionConfig` 完全适用于链接包场景 |
| `apiAdapters.ts` | **需要扩展**。`CreateLinkPayload.document_id` 改为 `document_ids: string[]` |

### 3.3 安全策略应用范围

**关键设计决策**: 一个链接包内的所有文档共享同一套安全策略。

理由：
- 降低用户配置负担（无需每个文档单独设置）
- 与实际场景一致（发给同一批投资人的资料包，安全要求一致）
- 后端实现简单（一个 bundle 一条 link 记录 + 多条 link_documents 关联）

如果未来需要文档级差异化权限 → 作为 v2 迭代。

### 3.4 交互规则

| 规则 | 说明 |
|:---|:---|
| **预设一键切换** | 点击预设卡片 → 安全选项自动填充，但保留已选的联系人（如果新预设需要邮箱验证） |
| **手动微调** | 修改任何安全选项 → 预设卡片显示 "自定义" 标签 + 实时重算评分 |
| **联系人必选** | 启用邮箱验证但未选联系人 → "下一步" 按钮 disabled + 内联提示 |
| **密码必填** | 启用密码但密码为空 → "下一步" 按钮 disabled + 内联提示 |
| **评分即时反馈** | 每次开关/输入变化 → 评分条和预设说明实时更新 |
| **上一步保留** | 返回 Step 1 安全配置不丢失 |

### 3.5 关键状态

```ts
interface BundleSecurityState {
  config: PermissionConfig;   // 复用现有类型
}
```

---

## 4. Step 3 — 完成（Review & Publish）

### 4.1 页面布局

```
┌──────────────────────────────────────────────────────┐
│  ← 返回安全设置                                       │
│                                                      │
│  确认并创建链接包                                     │
│  复核以下信息，确认后生成分享链接                       │
│                                                      │
│  ┌──────────────────────────────────────────────┐    │
│  │  📦 链接包内容 (2 篇文档)                     │    │
│  │  ┌──────────────────────────────────────┐    │    │
│  │  │ 1. 📄 商业计划书 v4.2           PDF  │    │    │
│  │  │ 2. 📄 尽调清单                  DOCX │    │    │
│  │  └──────────────────────────────────────┘    │    │
│  │                                              │    │
│  │  🔒 安全配置：[标准尽调]                       │    │
│  │  ┌──────────────────────────────────────┐    │    │
│  │  │ 邮箱验证 · 白名单 · 水印 · 禁止下载    │    │    │
│  │  │ 有效期 30 天 · 无访问次数限制          │    │    │
│  │  │ 安全强度 7/10 · 接收方摩擦 5/10       │    │    │
│  │  └──────────────────────────────────────┘    │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌─ 生成成功后 ─────────────────────────────────┐    │
│  │  ✅ 链接包已创建！                            │    │
│  │                                              │    │
│  │  ┌──────────────────────────────────────┐    │    │
│  │  │ https://deal.link/b/abc123xyz        │ [📋] │    │
│  │  └──────────────────────────────────────┘    │    │
│  │                                              │    │
│  │  [📋 复制链接]  [📧 通过邮件发送]  [🔗 查看链接]│    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌──────────────────────────────────────────────┐    │
│  │            ○ 选文档  —  ○ 设安全  —  ● 完成  │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│                    [ 上一步 ]  [ 🚀 创建链接包 ]      │
└──────────────────────────────────────────────────────┘
```

### 4.2 交互规则

| 规则 | 说明 |
|:---|:---|
| **创建前复核** | 清单列出所有已选文档 + 安全配置摘要，用户最终确认 |
| **创建按钮** | 「🚀 创建链接包」→ 调用 API → 显示结果 |
| **创建中** | 按钮 loading 态，显示 "创建中..." |
| **创建成功** | 替换确认区为成功卡片：显示链接 URL + 复制/邮件/查看操作 |
| **创建失败** | toast 错误提示，按钮恢复可点击，允许重试 |
| **复制链接** | `copyToClipboard` → 图标变 ✅ 2秒 |
| **通过邮件发送** | 跳转到邮件客户端 `mailto:` 预设链接，或内联邮件发送（future） |
| **查看链接** | 跳转到 `/links/:linkId` 详情页 |
| **再次创建** | 按钮变为 "创建新的链接包"，点击重置回到 Step 1 |
| **修改信息** | 点击 Step 1 或 Step 2 对应区域 → 返回对应步骤修改，状态不丢失 |

### 4.3 关键状态

```ts
interface BundlePublishState {
  isCreating: boolean;
  generatedLink: string | null;      // 链接包 URL
  copied: boolean;
}
```

---

## 5. 全局流水线机制

### 5.1 Pipeline 状态管理

用一个 **Context + Reducer** 管理跨步骤状态：

```ts
// apps/web/src/components/links/link-bundle/BundlePipelineContext.tsx

interface BundlePipelineState {
  step: 1 | 2 | 3;
  mode: 'create' | 'edit';           // ★ 创建 or 编辑
  editingLinkId: string | null;       // ★ 编辑模式下的 link ID
  linkToken: string | null;           // ★ link token（编辑模式下用于生成预览 URL）

  // Step 1 — 文档选择
  documents: Document[];
  selectedDocuments: Document[];
  searchQuery: string;

  // Step 2 — 安全配置（复用 PermissionConfig）
  config: PermissionConfig;

  // Step 3 — 提交
  isSubmitting: boolean;              // 提交中（创建 or 更新）
  generatedLink: string | null;       // link URL
  copied: boolean;
  isDirty: boolean;                   // ★ 编辑模式：是否有未保存修改
}

type BundlePipelineAction =
  | { type: "GO_STEP"; step: 1 | 2 | 3 }
  | { type: "INIT_FOR_EDIT"; documents: Document[]; config: PermissionConfig; linkId: string; token: string }
  | { type: "SET_DOCUMENTS"; documents: Document[] }
  | { type: "TOGGLE_DOCUMENT"; document: Document }
  | { type: "REMOVE_DOCUMENT"; documentId: string }
  | { type: "REORDER_DOCUMENTS"; fromIndex: number; toIndex: number }
  | { type: "SET_SEARCH_QUERY"; query: string }
  | { type: "SET_CONFIG"; config: PermissionConfig }
  | { type: "SET_SUBMITTING"; isSubmitting: boolean }
  | { type: "SET_GENERATED_LINK"; link: string | null }
  | { type: "SET_COPIED"; copied: boolean }
  | { type: "SET_DIRTY"; isDirty: boolean }
  | { type: "RESET" };
```

### 5.2 进度条

所有 3 个步骤页面底部统一渲染：

```
┌──────────────────────────────────────────────┐
│            ○ 选文档  —  ● 设安全  —  ○ 完成  │
│  Step 1              Step 2          Step 3  │
└──────────────────────────────────────────────┘
```

| 状态 | 样式 |
|:---|:---|
| 已完成（past） | 实心圆 + 主题色连线 |
| 当前（current） | 实心圆 + 脉冲动画 + 加粗文字 |
| 未完成（future） | 空心圆 + 灰色连线 |

点击已完成步骤的圆点可以直接跳回该步骤（快捷导航）。

### 5.3 导航规则

| 操作 | 行为 |
|:---|:---|
| Step 1 → Step 2 | `selectedDocuments.length >= 1` 方可点击 |
| Step 2 → Step 3 | 联系人必选验证 + 密码非空验证通过 |
| Step 2 → Step 1（返回） | 已选文档和安全配置均保留；**重新 GET /documents** 拉取最新列表（新上传的可见），与已选集合合并 |
| Step 3 → Step 2（返回） | 所有状态保留 |
| Step 3 → Step 1（返回×2） | 同上，重新拉取文档列表并合并已选 |
| 点击 Step 进度条圆点 | 跳转到对应步骤，已完成步骤可随意跳转；跳回 Step 1 时触发文档列表刷新 |
| 编辑模式 Step 3「保存修改」| `PUT /links/:id`，成功后 toast "已更新"，标记 `isDirty = false`；**接收方刷新 `/l/:token` 立即生效** |
| 点击全局「取消」| 编辑模式：如果有未保存修改 → `isDirty` 检查 → 弹出确认；创建模式：已选文档 > 0 → 弹出确认 |

### 5.4 路由设计

```
创建模式:
  /:workspaceSlug/links/new
    ?step=1 | ?step=2 | ?step=3     (可选，深度链接)
    ?documentId=xxx                 (兼容旧入口，预选单文档)

编辑模式:
  /:workspaceSlug/links/:id/edit
    ?step=1 | ?step=2 | ?step=3     (可选，默认进入 Step 1)
```

步骤间切换是**客户端状态变化**（不产生浏览器历史记录），进度条圆点点击 push/replace `?step=N` 以支持浏览器前进/后退。

### 5.5 创建模式 vs 编辑模式

| 维度 | 创建模式 (`/links/new`) | 编辑模式 (`/links/:id/edit`) |
|:---|:---|:---|
| 初始化 | 空文档列表 + 默认 standard 预设 | `GET /links/:id` 拉取当前文档列表 + 安全配置，回填 Context |
| Step 1 默认 | 无已选文档（除非 `?documentId`） | 当前链接包的全部已关联文档 |
| Step 2 默认 | standard 预设 | 当前链接的安全配置（内联字段展开） |
| 进度条标题 | "创建链接包" | "编辑链接包" |
| Step 3 操作按钮 | "创建链接包" | "保存修改" |
| API 调用 | `POST /links` | `PUT /links/:id` |
| 成功后 | 显示链接 URL + 复制 | 显示 "已更新" toast，跳回链接列表或保持在当前页 |
| 接收方影响 | 首次获得链接 | **刷新页面即生效**（实时拉取最新文档列表 + 安全配置）

---

## 6. 后端适配

### 6.1 API 变更

| 端点 | 变更 |
|:---|:---|
| `POST /links` | `document_id` → `document_ids: string[]`（必填，至少1个）。安全字段内联不变。 |
| `PUT /links/:id` | **新增**。入参与 `POST` 一致。更新 `link_documents`（增删文档）+ `links` 表安全字段。接收方下次请求 `/l/:token` 即拿到最新数据。 |
| `GET /links/:id` | 响应增加 `documents: { id, title, pageCount, sourceType }[]` — **实时关联查询 documents 表** |
| `GET /l/:token` | 公开访问端：实时查询 `link_documents` JOIN `documents` 获取最新文档列表 + 最新安全配置 |
| `Link` 类型 | 新增 `isBundle: boolean` / `documentIds: string[]` / `documents: DocumentSummary[]` |

### 6.2 数据库变更

```sql
-- link_documents 关联表（纯关联，无快照字段）
CREATE TABLE link_documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
  document_id UUID NOT NULL REFERENCES documents(id),
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(link_id, document_id)
);

CREATE INDEX idx_link_documents_link_id ON link_documents(link_id);
```

- `link_documents` 是**纯关联表**（不存储文档元数据快照），文档标题/页数/类型等全部实时从 `documents` 表 JOIN 查询。
- `PUT /links/:id` 更新时：`link_documents` 删除旧关联 + 批量插入新关联（DELETE + INSERT within transaction）。
- 兼容单文档：`document_ids: ["xxx"]` → `link_documents` 写入一条记录。
- 单文档旧链接：`links.document_id` 保留（backward compat），`link_documents` 迁移时回填。

### 6.3 公开访问端适配

```
访客打开 /l/:token：

┌─────────────────────────────────────────────┐
│         🔒 安全验证（一次性，链接包级别）      │
│    邮箱验证 → 白名单 → 密码 → NDA             │
│                                             │
│    安全配置每次请求实时从 links 表读取         │
│    ★ 修改后立即生效，无需额外操作              │
└──────────────────────┬──────────────────────┘
                       │ 验证通过
                       ▼
┌─────────────────────────────────────────────┐
│         文档列表（实时 JOIN 查询）            │
│                                             │
│  📄 商业计划书 v5.0                    PDF   │  ← 源文档已更新到 v5.0
│  📄 尽调清单                          DOCX   │
│  📄 市场分析报告                      PDF    │  ← 刚通过编辑新增的文档
│                                             │
│  点击进入阅读器 → 实时拉取最新文档内容         │
└─────────────────────────────────────────────┘

★ 所有文档内容、列表、安全策略均为实时查询
★ 创建者编辑链接后，接收方只需刷新页面即获最新
```

- 安全验证（邮箱/密码/NDA/白名单）在链接包级别执行一次，验证通过后所有文档可访问。
- 文档列表按 `link_documents.sort_order` 排列。
- 文档内容始终拉取 `documents` 表最新版本——无需"已更新" badge，因为始终是最新。

---

## 7. 前端文件结构

```
apps/web/src/components/links/
├── SmartLinkCreator.tsx          → [废弃] 替换为 BundlePipelinePage
├── LinksTable.tsx                → [保留] 表格增加"链接包"标签和文档数
├── LinkDetail.tsx                → [改造] 详情页展示文档列表
├── link-bundle/                  → [新增]
│   ├── BundlePipelinePage.tsx    → 路由入口，包裹 Provider + 进度条 + 步骤路由
│   ├── BundlePipelineContext.tsx → Context + Reducer
│   ├── PipelineProgress.tsx      → 3步进度条组件
│   ├── StepDocuments.tsx         → Step 1: 选文档页
│   ├── BundleDocumentPicker.tsx  → 多选文档列表 + 搜索 + 已选标签 + 拖拽排序
│   ├── StepSecurity.tsx          → Step 2: 设安全页
│   ├── StepReview.tsx            → Step 3: 复核 & 发布页
│   └── BundleLinkPreview.tsx     → 生成成功卡片（链接 URL + 操作按钮）
├── smart-link/                   → [保留，StepSecurity 内部引用]
│   ├── PermissionPanel.tsx       → 原样复用
│   ├── SecurityOptions.tsx       → 原样复用
│   ├── ScoreDisplay.tsx          → 原样复用
│   ├── ContactSelector.tsx       → 原样复用
│   ├── LinkPreview.tsx           → [不再需要，被 BundleLinkPreview 替代]
│   ├── DocumentSelector.tsx      → [不再需要，被 BundleDocumentPicker 替代]
│   ├── levelConfig.ts            → 零改动
│   └── types.ts                  → 零改动
```

---

## 8. i18n 扩展

在 `links.json` (en/zh-CN) 中新增以下 key：

```json
{
  "bundle": {
    "titleCreate": "Create Link Bundle",
    "titleEdit": "Edit Link Bundle",
    "subtitle": "Select multiple documents, configure security, and share as one link.",
    "stepDocuments": "Select Documents",
    "stepSecurity": "Security",
    "stepReview": "Review & Publish",
    "nextStep": "Next",
    "prevStep": "Back",
    "cancelConfirmTitle": "Discard changes?",
    "cancelConfirmDesc": "Your changes will be lost.",
    "documents": {
      "label": "Documents",
      "searchPlaceholder": "Search documents...",
      "selectedCount": "{{count}} document(s) selected",
      "dragToReorder": "Drag to reorder",
      "empty": "No documents selected",
      "minRequired": "Select at least one document"
    },
    "review": {
      "titleCreate": "Review & Create",
      "titleEdit": "Review & Save Changes",
      "subtitle": "Review before saving",
      "documentsSection": "Link Bundle Contents",
      "securitySection": "Security Configuration",
      "createButton": "Create Link Bundle",
      "saveButton": "Save Changes",
      "submitting": "Saving...",
      "successCreate": "Link bundle created!",
      "successUpdate": "Link updated! Recipients will see the latest changes on refresh.",
      "viewLink": "View Link",
      "sendViaEmail": "Send via Email",
      "createAnother": "Create Another Bundle",
      "backToList": "Back to Links"
    }
  }
}
```

中文对应：

```json
{
  "bundle": {
    "title": "创建链接包",
    "subtitle": "选择多篇文档，统一配置安全策略，生成一个分享链接。",
    "stepDocuments": "选文档",
    "stepSecurity": "设安全",
    "stepReview": "确认发布",
    "nextStep": "下一步",
    "prevStep": "上一步",
    "cancelConfirmTitle": "放弃编辑？",
    "cancelConfirmDesc": "当前配置将不会被保存。",
    "documents": {
      "label": "文档",
      "searchPlaceholder": "搜索文档...",
      "selectedCount": "已选 {{count}} 篇文档",
      "dragToReorder": "拖拽调整顺序",
      "empty": "未选择任何文档",
      "minRequired": "请至少选择一篇文档",
      "updatedBadge": "已更新",
      "deletedBadge": "已删除",
      "newlyAvailable": "新增可用"
    },
    "review": {
      "title": "确认并创建",
      "subtitle": "确认信息后生成分享链接",
      "documentsSection": "链接包内容",
      "securitySection": "安全配置",
      "createButton": "创建链接包",
      "creating": "创建中...",
      "success": "链接包已创建！",
      "viewLink": "查看链接",
      "sendViaEmail": "通过邮件发送",
      "createAnother": "创建新的链接包",
      "documentUpdated": "「{{oldTitle}}」已更新为「{{newTitle}}」",
      "documentDeleted": "「{{title}}」已被删除",
      "someDocumentsChanged": "{{count}} 篇文档在选择后发生了变化，链接将使用最新版本。"
    }
  }
}
```

---

## 9. 视觉与交互规范

### 9.1 进度条动画

- 当前步骤圆点：`scale(1.2)` + `box-shadow` 脉冲动画（`@keyframes pulse-step`）
- 步骤切换：`motion.div` 左右滑入/滑出（`animate={{ x: direction * 20, opacity: 0 }} → { x: 0, opacity: 1 }`）
- 已完成连线：`bg-primary` 过渡动画

### 9.2 响应式

- 桌面（≥1024px）：Step 2 采用左右双栏（预设卡片 + 评分在右）
- 平板/手机（<1024px）：全部单栏堆叠，评分卡片移到预设上方

### 9.3 空状态

- 无文档：引导上传按钮
- 无搜索结果：「没有匹配"xxx"的文档」
- 已选 0 篇：下一步按钮 disabled + tooltip

---

## 10. 交互原型关键路径

```
进入 /links/new
  │
  ▼
┌─────────┐   选择文档(≥1)    ┌─────────┐   配置安全+验证通过   ┌─────────┐
│ Step 1  │ ───────────────→ │ Step 2  │ ──────────────────→ │ Step 3  │
│ 选文档   │ ←─────────────── │ 设安全   │ ←────────────────── │ 完成     │
└─────────┘   "上一步"        └─────────┘   "上一步"           └─────────┘
     ▲            ▲                                               │
     │            │                           点击「创建链接包」     │
     │            │                                               ▼
     │            │                                      ┌─────────────┐
     │            │                                      │ 成功卡片      │
     │            │                                      │ 复制 / 邮件  │
     │            │                                      │ 查看 / 再创建 │
     │            │                                      └─────────────┘
     │            │
     │   ┌────────┴────────┐
     │   │ 返回 Step 1 时   │
     │   │ 自动重新拉取最新  │  ← 发现新上传文档 → 可补充选择
     │   │ 文档列表          │
     │   └─────────────────┘
     │
     └── 进度条圆点点击直接跳回（同样触发列表刷新）

进入 Step 3 时额外触发:
     GET /documents/check-updates
     → 检测已选文档是否有内容更新
     → 有更新 → 非阻塞提示（不阻断流程）
     → 无更新 → 静默通过
```

---

## 6.4 数据一致性：Live Reference 模型（实时引用）

### 6.4.0 核心模型

**已创建的分享链接是可编辑的**。创建者可在链接列表中点击「编辑」进入流水线，增删文档或调整安全策略，保存后立即生效。接收方刷新页面即拿到最新数据。

不再需要快照（Snapshot）——所有数据都是实时的：

```
┌─────────────────────────────────────────────────┐
│              创建模式（流水线编辑器）              │
│      所有数据 LIVE — 实时反映最新状态              │
│      刷新 → 重新拉取最新文档、最新预设定义         │
│      无持久化草稿，刷新即丢弃                      │
└──────────────────────┬──────────────────────────┘
                       │ 点击「创建链接包」
                       ▼
┌─────────────────────────────────────────────────┐
│              ★ 已创建（可编辑，实时生效）          │
│      文档关联 → link_documents（纯关联表）        │
│      安全配置 → links 表内联字段                  │
│      随时可编辑 → 修改后立即对接收方生效           │
│      接收方刷新 = 最新文档 + 最新安全策略           │
└─────────────────────────────────────────────────┘
                       │ 点击「编辑」
                       ▼
┌─────────────────────────────────────────────────┐
│              编辑模式（流水线编辑器）              │
│      GET /links/:id 回填文档+安全配置             │
│      增删文档 / 调整安全配置                      │
│      保存 → PUT /links/:id → 实时生效             │
└─────────────────────────────────────────────────┘
```

### 6.4.1 三条数据线的处理

| 数据线 | 创建中（LIVE） | 创建后 | 编辑时 | 接收方 |
|:---|:---|:---|:---|:---|
| **文档集合** | 实时从 `documents` 表选取 | `link_documents` 存关联（仅 `link_id + document_id + sort_order`） | 重新 GET /documents 列表 + 回填已关联，可增删改排序 | 实时 JOIN `link_documents` + `documents` |
| **文档内容** | 用户看到的是文档最新版本 | —（不存快照） | —（不存快照） | **始终拉取 `documents` 表最新内容** |
| **安全配置** | `levelConfig.ts` 驱动 | `links` 表内联字段（`require_email_verification` 等） | 读取 `links` 表现有值展开到 `PermissionConfig`，用户可修改 | 实时读取 `links` 表 |
| **预设名称** | 实时匹配 | 存为 `permission_preset`（仅展示，编辑时可改为新预设） | 同创建模式 | 仅用于展示 |

### 6.4.2 各场景行为矩阵

| 场景 | 流水线内 | 已创建链接（接收方 `/l/:token`） |
|:---|:---|:---|
| 新文档上传 | 刷新后出现在可选列表 ✅ | 不自动出现在已发链接中（需创建者编辑链接补充） |
| 文档内容更新 | 选择的是最新内容 ✅ | 刷新页面即看到最新内容 ✅（实时 JOIN） |
| 文档被删除 | 已选列表显示"已删除"标签，需移除后才可提交 | 该文档从链接中消失（需要创建者编辑移除） |
| 安全预设定义变更 | 使用最新 levelConfig 定义 | 已创建链接不受影响（内联字段），除非创建者手动编辑 |
| 创建者编辑链接 | — | PUT 成功后立即生效，接收方刷新即可见 ✅ |
| 接收方刷新页面 | — | 所有数据实时拉取，始终是最新 ✅ |

### 6.4.3 编辑链接的入口

```
链接列表页 /links
  │
  ├── 每行操作菜单增加「编辑」按钮 → /links/:id/edit
  │
链接详情页 /links/:id
  │
  └── 顶部「编辑」按钮 → /links/:id/edit
```

### 6.4.4 编辑模式的 Dirty 检测与离开保护

```
编辑模式下：

进入 /links/:id/edit
  │
  ├── INIT_FOR_EDIT: isDirty = false
  │
  ├── 用户修改文档选择 or 安全配置
  │     → SET_DIRTY: isDirty = true
  │
  ├── 用户点击「取消」
  │     ├── isDirty = false → 直接返回列表
  │     └── isDirty = true  → 弹出确认框："有未保存的修改，确定离开？"
  │
  └── beforeunload 事件:
        isDirty = true → preventDefault + 浏览器原生确认框
```

### 6.4.5 API Payload（创建和编辑共用结构）

```ts
// apiAdapters.ts — CreateLinkPayload / UpdateLinkPayload 共用
export interface LinkPayload {
  document_ids: string[];          // ★ 所有场景改为数组
  sort_order: {                    // ★ 文档展示顺序
    document_id: string;
    order: number;
  }[];

  // Security（内联字段）
  require_email_verification: boolean;
  require_password: boolean;
  require_nda: boolean;
  allowed_emails?: string[];
  allowed_domains?: string[];
  password?: string;
  contact_ids?: string[];
  download_enabled: boolean;
  watermark_enabled: boolean;
  expires_at?: string;
  max_access_count?: number;

  // 展示用
  permission_preset: string;
}
```

### 6.4.6 为什么是 Live Reference 而非 Snapshot

| 对比维度 | Live Reference（当前选择） | Snapshot（废弃方案） |
|:---|:---|:---|
| 创建者可编辑 | ✅ 随时补充文档、调整安全 | ❌ 创建即冻结，只能新建 |
| 接收方看到的内容 | 始终最新 | 创建时的版本（有"已更新"提示） |
| 存储成本 | 纯关联表，3 列 | 需要 `snapshot_title/page_count/hash` 存储 |
| 复杂度 | 低：读时 JOIN，写时 DELETE+INSERT | 中：快照字段管理 + 变化检测逻辑 |
| 文档删除处理 | 创建者编辑移除即可 | 需软降级展示，无法自动清理 |
| 用户场景覆盖 | 覆盖"漏发文件补充""安全策略纠错" | 不支持编辑，只能重建 |

> **关键洞察**：投行/FA 的真实场景是"材料持续完善、反复沟通"——链接发出后发现漏了财务模型、需要调整水印策略是常见需求。Live Reference 模型让同一个链接成为持续沟通的载体，而非一次性快照。

---

## 11. 开发排期建议

| 阶段 | 范围 | 估时 |
|:---|:---|:---|
| Phase 1 | 后端 `link_documents` 表（含快照字段）+ snapshot API 改造 | 2d |
| Phase 2 | Pipeline Context + 进度条 + Step 容器框架 + beforeunload 保护 | 1d |
| Phase 3 | Step 1 — BundleDocumentPicker + 搜索 + 多选 + 拖拽排序 | 1.5d |
| Phase 4 | Step 2 — 复用安全组件，左右双栏布局适配 | 1d |
| Phase 5 | Step 3 — 复核页 + 快照确认 + 创建成功卡片 | 1d |
| Phase 6 | 公开访问端文档列表 + 已更新/已删除状态处理 | 1d |
| Phase 7 | i18n + 边界情况 + E2E 测试 | 1d |
| **合计** | | **8.5d** |

---

## 附录 A. 设计决策记录

| 决策 | 选择 | 理由 |
|:---|:---|:---|
| 单包单安全策略 vs 文档级策略 | 单包单策略 | 降低复杂度，与场景一致，v2 可视需求扩展 |
| 3步流水线 vs Tab 切换 | 3步流水线 | Pipeline 引导性强，降低单页认知负担；Tab 切换缺乏完成感 |
| 步骤间客户端状态 vs 路由 | 客户端状态为主，可选?step=N | 避免不必要的历史条目堆积，但支持深度链接 |
| 进度条可点击跳转 | 是 | 已完成步骤可自由跳回修改，符合用户预期 |
| 拖拽排序 vs 手动上下移动 | 拖拽排序 | 交互直觉，`@dnd-kit` 成熟方案 |
| 文档快照 vs 文档版本化 vs Copy-on-Publish | **Snapshot Metadata** | 元数据快照成本为零；内容变化时"仍可用+提示"体验最优；真正需要版本锁定的场景是 v2 功能 |
| 流水线草稿持久化 vs 刷新即丢弃 | **刷新即丢弃 + beforeunload 保护** | 创建是短时操作，不持久化=天然获得"刷新即最新"；beforeunload 防止误操作丢失 |
| 安全预设存储方式 | **内联字段 + 只读 preset 名** | 预设定义漂移不影响已创建链接；preset 名仅用于详情页展示 |

## 附录 B. 风险与缓解

| 风险 | 缓解 |
|:---|:---|
| 后端改造范围超出预期 | `link_documents` 表设计兼容单文档，可增量迁移 |
| 拖拽在移动端体验差 | 移动端降级为"上移/下移"按钮 |
| 大文档量搜索性能 | 前端过滤 + `useDeferredValue` 延迟更新，>500 篇考虑虚拟列表 |
| 旧 `SmartLinkCreator` 废弃影响 | 保留旧代码但路由指向新组件，灰度切换 |
| 文档删除导致链接包断裂 | 快照元数据保留删除时的 title，前端软降级显示"已删除" |
| 内容更新后 hash 对比失败误报 | `snapshot_content_hash` 为可选字段，初期可不启用，仅依赖元数据快照 |
