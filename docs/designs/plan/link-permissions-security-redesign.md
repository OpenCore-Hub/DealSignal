# 创建链接 Permissions & Security 重构设计方案

> 版本：v1.0
> 日期：2026-07-01
> 状态：已批准，待实施

## 一、背景与目标

### 1.1 背景

当前创建链接的 Permissions & Security 中存在「Require email」和「Require email verification code」两个选项。前者仅收集邮箱字符串，后者通过 6 位验证码验证邮箱。两者功能重叠，且后者无法与现有联系人系统打通。

### 1.2 目标

1. 移除「Require email」选项，简化用户决策。
2. 将「Require email verification code」与联系人系统打通：
   - 勾选后显示联系人选择器。
   - 支持搜索/选择已有联系人，或新增联系人。
   - 新增联系人提交后自动返回并选中。
3. 创建链接时，若启用「Require email verification code」，后端立即向所选联系人邮箱发送专属查看码。
4. 查看码不设独立过期时间，与链接本身的 `expires_at` 对齐。

---

## 二、用户画像与心智模型

### 2.1 用户画像

| 角色 | 场景 | 诉求 |
|------|------|------|
| B2B 销售 Alice | 给 3 位投资人发送融资材料 | 只发给已录入 CRM 的联系人，防止转发泄露 |
| 法务 Bob | 向外部律师发送尽调文档 | 避免手动输入邮箱出错，希望从联系人库选择 |
| 市场 Carol | 给活动报名用户发白皮书 | 批量选择联系人，自动发送查看码 |

### 2.2 心智模型

- "我要把这份文档分享给谁" → 选择联系人。
- "我要不要验证对方身份" → 开启验证码。
- "我不希望随便谁都能看" → 开启联系人白名单/密码/NDA。

### 2.3 设计原则

1. **以联系人为中心**：分享 = 选择收件人 + 设置权限。邮箱不再作为自由文本输入。
2. **权限等级清晰**：
   - Low：公开链接（anyone with the link）
   - Medium：需指定联系人 + 邮箱查看码
   - High：联系人白名单 + 密码 + NDA
3. **减少重复**：验证码已强制收集邮箱，因此不再提供「Require email」。
4. **流畅添加联系人**：联系人不存在时，一键新增并自动选中。

---

## 三、端至端流程

### 3.1 创建链接流程

1. 用户进入 `/:workspaceSlug/links/new`。
2. 选择文档。
3. 调整 Permissions & Security：
   - 选择 Level：Low / Medium / High。
   - 勾选「Require email verification code」后：
     - 显示联系人选择器（Combobox）。
     - 支持搜索已有联系人。
     - 底部显示「+ New contact」按钮。
   - 点击「+ New contact」跳转至联系人添加页面。
   - 新增联系人提交后自动返回链接创建页并选中该联系人。
4. 点击「Create Link」。
5. 后端创建 `link_contacts` 关联，并为每位联系人生成唯一 6 位查看码。
6. 后端立即调用邮件服务，向所选联系人发送查看码邮件。
7. 前端展示成功状态：
   - 生成的 shortUrl。
   - 已选联系人列表及发送状态。
   - 复制链接按钮。

### 3.2 访问链接流程

1. 访客打开 `/l/:token`。
2. 如果链接开启「Require email verification code」：
   - 显示邮箱输入框，提示"请输入您的邮箱"。
   - 显示 6 位查看码输入框。
   - 提供「Resend code」按钮。
3. 访客输入邮箱和查看码，点击「Continue」。
4. 后端校验：
   - 邮箱属于 `link_contacts` 中的联系人。
   - 查看码匹配。
   - 查看码未使用过（`used_at IS NULL`）。
5. 校验通过后标记 `used_at = now()`，该码失效。
6. 如果同时开启密码/NDA，继续校验。
7. 签发 `LinkSession`，访客查看文档。

### 3.3 重新发送查看码流程

1. 访客在访问页点击「Resend code」。
2. 输入邮箱并提交。
3. 后端校验邮箱属于 `link_contacts`。
4. 重新生成查看码，更新 `link_contacts.access_code` 和 `code_sent_at`。
5. 发送新查看码邮件。
6. 前端提示"新查看码已发送"。

---

## 四、Permissions & Security UI 详细设计

### 4.1 安全选项变更

移除「Require email」复选框，只保留「Require email verification code」。

勾选「Require email verification code」后：
- 必须选择至少一个联系人。
- 访客访问时必须用联系人的邮箱和查看码验证身份。

### 4.2 联系人选择器

**组件位置**：`SecurityOptions` 中「Require email verification code」下方。

**交互**：
- 未勾选时隐藏。
- 勾选后显示 Combobox：
  - 顶部搜索框，实时过滤联系人（按 name/email 匹配）。
  - 联系人列表：每项显示 name + email。
  - 底部分隔线 + 「+ New contact」按钮。
- 选择后显示为 chip，可删除。
- MVP 支持单选；后续可扩展多选。

**空状态**：
- 如果没有联系人，下拉列表仅显示「+ New contact」。
- 提示文案："No contacts yet. Add a contact to send the access code."

### 4.3 权限等级映射

| Level | 触发条件 | 默认勾选 |
|-------|---------|---------|
| Low | 无安全选项 | 无 |
| Medium | `requireEmailVerification` 开启 | `requireEmailVerification` |
| High | `passwordEnabled` / `whitelistEnabled` / `ndaEnabled` 任一开启 | 上述全部 |

**变更点**：
- `deriveLevelFromConfig` 移除 `requireEmail`，改为 `requireEmailVerification`。
- `handleLevelChange`：
  - low：关闭所有安全选项，清空已选联系人。
  - medium：开启 `requireEmailVerification`。
  - high：开启 `whitelistEnabled`、`passwordEnabled`、`ndaEnabled`、`watermarkEnabled`。

### 4.4 LinkPreview 更新

创建成功后展示：
- 生成的 shortUrl。
- 已选联系人列表（name + email）及"Code sent"状态。
- 特性标签：
  - Email verification code
  - Password
  - Watermark
  - No download

---

## 五、数据模型设计

### 5.1 新增 `link_contacts` 关联表

**迁移文件**：`apps/api/internal/db/migrations/027_link_contacts.up.sql`

```sql
CREATE TABLE IF NOT EXISTS link_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    access_code TEXT NOT NULL,
    code_sent_at TIMESTAMPTZ DEFAULT now(),
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(link_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_link_contacts_link ON link_contacts(link_id);
CREATE INDEX IF NOT EXISTS idx_link_contacts_contact ON link_contacts(contact_id);
```

**字段说明**：
- `link_id`：关联的链接。
- `contact_id`：关联的联系人。
- `access_code`：该联系人的专属 6 位查看码。
- `code_sent_at`：查看码发送时间。
- `used_at`：首次使用时间（一次性使用）。

### 5.2 `contacts` 表

保持不变。现有字段：`id`, `workspace_id`, `email`, `name`, `organization`, `role`, `created_at`。

### 5.3 `links` 表

保留 `require_email_verification` 字段。`require_email` 字段数据库保留但前端不再使用，避免破坏已有数据。

---

## 六、后端技术方案

### 6.1 新增联系人创建 API

**端点**：

```
POST /api/workspaces/:workspaceSlug/contacts
```

**请求体**：

```json
{
  "email": "alice@example.com",
  "name": "Alice Chen",
  "organization": "Acme Capital",
  "role": "Partner"
}
```

**响应**：创建的 Contact 对象。

**实现**：
- `internal/contact/service.go` 新增 `CreateContact` 方法。
- `internal/contact/handler.go` 新增 `Create` handler。
- 校验 email 格式。
- 校验 workspace 内 email 唯一（依赖数据库唯一约束）。

### 6.2 链接创建 API 变更

**端点**：

```
POST /api/workspaces/:workspaceSlug/links
```

**请求体增加**：

```json
{
  "contact_ids": ["uuid-1"]
}
```

**后端处理**：
1. 如果 `require_email_verification` 为 true：
   - 校验 `contact_ids` 非空。
   - 校验所有 contact 属于当前 workspace。
2. 创建 `links` 记录。
3. 创建 `link_contacts` 关联，为每个联系人生成唯一 6 位 `access_code`。
4. 遍历关联联系人，调用 `mailer.SendLinkAccessCodeEmail` 发送查看码邮件。

### 6.3 查看码生命周期

- **生成**：创建链接时为每个 `link_contact` 生成 6 位数字码。
- **存储**：存在 `link_contacts.access_code`。
- **有效期**：与链接本身 `expires_at` 对齐，不单独设置过期时间。如果链接不过期，则查看码长期有效。
- **使用**：访客首次访问输入正确邮箱+查看码后，标记 `used_at = now()`，之后该码失效。
- **重新发送**：访客可在访问页点击「Resend code」，后端重新生成并发送新码。

### 6.4 访问流程变更

`POST /api/v1/public/links/:publicToken`

1. 校验 link 状态、过期时间、访问次数。
2. 如果 `require_email_verification`：
   - 校验 email 非空。
   - 查询 `link_contacts` 中该邮箱对应的记录。
   - 校验 `access_code` 匹配。
   - 校验 `used_at` 为空（未使用过）。
   - 标记 `used_at = now()`。
3. 继续 password/NDA 校验。
4. 签发 `LinkSession`。

**新增查询**：
- `GetLinkContactsByPublicToken(ctx, token) -> []LinkContactRow`
- `VerifyLinkContactCode(ctx, token, email, code) -> (ok, used bool)`
- `MarkLinkContactCodeUsed(ctx, token, email)`

### 6.5 重新发送查看码 API

**端点**：

```
POST /api/v1/public/links/:publicToken/resend-code
```

**请求体**：

```json
{
  "email": "alice@example.com"
}
```

**逻辑**：
1. 校验 link 存在且开启 `require_email_verification`。
2. 校验 link 未过期。
3. 校验 email 属于 `link_contacts`。
4. 重新生成 `access_code`。
5. 更新 `link_contacts.access_code` 和 `code_sent_at`。
6. 发送新查看码邮件。

### 6.6 邮件模板

`SendLinkAccessCodeEmail(to, code, linkName, linkURL)` 内容：

```
Hello,

{linkName} has been shared with you.

Your access code is: {code}

View the document at:
{linkURL}

This code is valid until the link expires and can only be used once.

If you did not request access, you can safely ignore this email.
```

---

## 七、前端技术方案

### 7.1 类型更新

`PermissionConfig` 移除 `requireEmail`，新增 `contactId`：

```ts
export interface PermissionConfig {
  level: "low" | "medium" | "high";
  requireEmailVerification: boolean;
  contactId?: string;
  whitelistEnabled: boolean;
  whitelist: string[];
  passwordEnabled: boolean;
  password?: string;
  ndaEnabled: boolean;
  allowDownload: boolean;
  watermarkEnabled: boolean;
  expiryDays: number | "custom";
  maxViews: number | "unlimited";
}
```

### 7.2 新增 ContactSelector 组件

**文件**：`apps/web/src/components/links/smart-link/ContactSelector.tsx`

**Props**：

```ts
interface ContactSelectorProps {
  workspaceSlug: string;
  selectedContactId?: string;
  onChange: (contactId?: string) => void;
  onCreateNew: () => void;
}
```

**功能**：
- 调用 `api.getContacts()` 加载联系人列表。
- 支持本地搜索（name/email）。
- 显示「+ New contact」按钮。
- 选中后显示 ContactChip，可删除。

### 7.3 新增联系人页面

**MVP 方案**：新建页面 `/:workspaceSlug/contacts/new`。

**理由**：
- 与现有联系人路由一致。
- 可复用表单验证和布局。
- 返回时通过 navigation state 传递新联系人 ID。

**实现**：
- 路由：`apps/web/src/router.tsx` 增加 `/:workspaceSlug/contacts/new`。
- 页面：`apps/web/src/routes/contacts/new.tsx`。
- 表单字段：name*、email*、organization、role。
- 提交后调用 `api.createContact(workspaceSlug, data)`。
- 成功后 `navigate(-1, { state: { newContactId: contact.id } })`。
- `SmartLinkCreator` 读取 `location.state?.newContactId` 并自动选中。

### 7.4 SecurityOptions 重构

- 移除「Require email」复选框。
- 「Require email verification code」勾选后显示 `ContactSelector`。
- 未选择联系人时显示错误提示："Please select a contact to send the access code."

### 7.5 PublicViewerPage 更新

如果开启 `requireEmailVerification`：
- 显示邮箱输入框。
- 显示查看码输入框。
- 提供「Resend code」按钮。
- 不再实时「Send code」（因为创建时已发送）。
- 文案提示："A 6-digit access code was sent to your email. Enter it below."

### 7.6 API 客户端更新

`api.ts`：
- 新增 `createContact(workspaceSlug, data)`。
- 新增 `getContacts(workspaceSlug)`（如果尚未存在）。
- 新增 `resendEmailVerificationCode(token, email)`。
- `createLink` payload 增加 `contact_ids`。
- `accessPublicLink` 支持 `emailCode`。

`apiAdapters.ts`：
- `toCreateLinkPayload` 移除 `require_email` 映射。
- 增加 `contact_ids: config.contactId ? [config.contactId] : undefined`。
- `require_email_verification: config.requireEmailVerification`。

### 7.7 i18n 更新

`links.json`：
- 移除 `requireEmail`。
- 新增：
  - `selectContact`
  - `newContact`
  - `noContacts`
  - `contactRequired`
  - `codeSentTo`

`documents.json`：
- 调整访客页面提示：
  - `checkEmailForCode`
  - `codeLabel`
  - `codePlaceholder`
  - `resendCode`
  - `codeResent`

---

## 八、API 变更汇总

### 新增

- `POST /api/workspaces/:workspaceSlug/contacts` — 创建联系人。
- `POST /api/v1/public/links/:publicToken/resend-code` — 重新发送查看码。
- 数据库表 `link_contacts`（含 `access_code`, `code_sent_at`, `used_at`）。
- sqlc query：
  - `CreateLinkContact`
  - `GetLinkContactsByPublicToken`
  - `VerifyLinkContactCode`
  - `MarkLinkContactCodeUsed`

### 修改

- `POST /api/workspaces/:workspaceSlug/links`
  - 请求体增加 `contact_ids`。
  - 创建时生成并发送查看码。
- `POST /api/v1/public/links/:publicToken`
  - 校验邮箱+查看码是否匹配 `link_contacts`。

### 移除

- 前端「Require email」选项（后端 `require_email` 字段保留兼容）。

---

## 九、实施步骤

1. **数据库**
   - 创建 `027_link_contacts.up.sql` 迁移。
   - 更新 `queries.sql`，增加 `link_contacts` 相关 query。
   - 运行 `sqlc generate`。

2. **后端**
   - 实现 `CreateContact` service + handler。
   - 注册 `POST /contacts` 路由。
   - 修改 `CreateLink`：支持 `contact_ids`，生成并发送查看码。
   - 修改 `Access`：校验 `link_contacts` 中的邮箱+查看码。
   - 新增 `ResendCode` handler。

3. **前端**
   - 更新 `PermissionConfig` 类型，移除 `requireEmail`。
   - 创建 `ContactSelector` 组件。
   - 创建 `contacts/new` 页面。
   - 更新 `SecurityOptions`，移除 `requireEmail`，集成 `ContactSelector`。
   - 更新 `SmartLinkCreator` 的 level 映射和自动选中返回联系人。
   - 更新 `PublicViewerPage`：显示查看码输入和重发按钮。
   - 更新 `api.ts` 和 `apiAdapters.ts`。
   - 更新 i18n。

4. **测试**
   - 后端：contact service/handler 测试，link 创建和访问测试。
   - 前端：`SecurityOptions` 渲染测试，`ContactSelector` 测试。
   - E2E：创建链接 → 新增联系人 → 发送验证码 → 访问链接。

---

## 十、风险与注意事项

1. **邮件发送失败**：创建链接时如果某个联系人邮件发送失败，应返回部分成功状态，前端提示用户。
2. **旧数据兼容**：`links.require_email` 字段数据库保留，但前端不再使用。
3. **验证码暴力破解**：虽然查看码只有 6 位，但攻击者需要先知道关联联系人的邮箱。可考虑对访问接口做 rate limit。
4. **多选扩展**：当前 MVP 为单选，后续如需多选，需调整 `contactId` 为 `contactIds` 数组。
