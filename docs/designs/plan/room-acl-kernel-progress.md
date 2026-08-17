# Room ACL kernel · 进度

日期：2026-08-14  
分支：`feat/room-acl-kernel`  
对照计划：Workspace Room Roles（方案 B · 两平面角色）  
产品锁定：两平面同名不同权；owner 只继承不授予；workspace owner/admin 对未加入的房间为 **oversight（可看不可改）**；席位只计 workspace `owner|admin|member` + pending `admin|member` 邀请，不计 `room_members`。

**总判定：** 方案 B 内核已收口（切片 1–10）。P2/P3 产品面与 P4 上线闸门（含 GetBilling 满席对照、巡视横幅、邀请失败关闭、`not_room_admin`）已落地。未提交。

### 原计划对照

| 项 | 原范围 | 状态 |
|---|---|---|
| P0 | 角色契约 + `contributor`/`viewer` 迁移 | **完成** |
| P0b | 房间邀请不写内部席；`CountInternalSeats` 不读 `room_members` | **完成** |
| P1 | `roomacl` + 去掉工作区 manager 写短路 | **完成** |
| P1b | 登录绑 `user_id`；唯一室主不可移出工作区 | **完成** |
| P2 | 数据室 Members 页签 | **完成**：水平页签 + 邀请/名单；Settings 只留 NDA/审批/室信息 |
| P3 | 工作区成员页文案区分两层 | **完成**：去 owner 回退；工作区角色带平面前缀；席位不计室成员 |
| P4 | 角色回归 + 席位对照 | **完成（上线闸门）**：含 `GetBilling.seatsUsed` 满席邀室 member；全量 link IT 仍不作为本分支门 |

---

## 已完成

### Slice 1 — 后端内核

- `roomacl` 单一能力解析：`NeedView|Contribute|Manage|Delete`；oversight 仅 View。
- 迁移 170：`contributor→member`、`viewer→guest`；房间角色 CHECK `owner|admin|member|guest`。
- 列表按可见房间收敛（`ROOM_LIST_SCOPED` 默认开）；oversight 看全部。
- 房间邀请不再写入 workspace `member|admin`，至多自动 workspace guest。
- 移除 workspace 成员时，在席位锁内拦截「房间唯一可操作者」。
- Knowledge / 房间级 share link Ask Host 走房间能力，不再用 workspace `canWrite` 短路。

### Slice 2 — 前端能力接线

- 房间详情/列表契约：`isAdmin`、`oversight`、`canContribute`。
- UI 按房间能力闸门，不再用 workspace `canWrite` 决定 Access / 邀请 / 文档写入。
- 邀请默认角色 `guest`。

### Slice 3 — Settings 成员名单

- 详情增加 `roomRole`，前端按 actor 房间角色计算可授予集合（owner → admin/member/guest；admin → member/guest）。
- Settings 展示成员列表：oversight / 只读可见；`canManage` 可改角色、移除。
- `PATCH /deal-rooms/:id/members/:memberId` 客户端 + MSW。
- `RemoveMember` 与改角色对齐：不可动 owner、不可改/移自己、admin 不可管理另一 admin。
- 错误码：`cannot_manage_member`、`invalid_role`；`cannot_remove_sole_room_operator` 已有 i18n。

### Slice 4 — Ask Host / Formal publish 闸门

- 后端 Ask Host 本就按 `NeedManage`（房间）/ workspace owner|admin（无 dealRoomID 的文档 link）。
- 认证 link 响应增加 `canManageAsk`，与 `authorizeAskHostOwnerView` 对齐。
- 前端不再用 workspace `canWrite` 显示回复 / Formal publish / pin FAQ。
- 房间级：房间 owner/admin 可见操作；oversight 只看到说明，不发 403 请求。
- 文档分享 link：workspace owner/admin（`canManage`），workspace member 不再看到会 403 的按钮。

### Slice 5 — 文件夹树 contribute vs manage

- `RemoveDocument` 改为 `NeedManage`；`AddDocument` / `MoveDocument` / `ReorderDocuments` 仍 `NeedContribute`。
- 文件夹树拆开闸门：成员可选中文件夹并批量上传；建/删目录、删文件、锁仍需 manage。`isAdmin` 作为 `canManage` 别名保留。
- 详情页传入 `canManage` + `canContribute`，不再只用 `isAdmin={canManage}` 挡住成员上传。

### Slice 6 — 房间级 share link 编辑/启停

- HTTP 层对 PATCH/PUT/DELETE/archive/renew/ask-policy 走 `authorizeLinkMutate`：房间 link 需 `NeedManage`（oversight 不可改）；文档 link 拒绝 workspace guest，其余仍靠 RBAC `write_content`。
- RBAC 允许 workspace guest 进入 `/links/:id` 条目（与 deal-room scoped mutate 同模式），**不允许** `POST /links` 建文档分享。文档 link 的 guest 写入由 `authorizeLinkMutate` 失败关闭。
- 前端 `canMutateShareLink`：房间 link 跟 Ask Host 同一能力；文档 link 仍 `canWrite`。Link 详情编辑/启停不再用 workspace `canWrite` 一把梭。

### Slice 7 — 其余 link 条目 mutate + 房间申请审核

- access-rules / invitations / revoke / access-code resend / generate-index / file-request status / uploaded-file approve|reject 均走 `loadLinkForMutate`。上传文件与 file-request 必须属于路径上的 link。
- 文档 link 申请审核仍仅 creator；房间 link 改为 `NeedManage`（房间 owner/admin 可审自己没创建的 link）。oversight GET 空列表、mutate 403。
- Deal-room pending inbox SQL 去掉 `created_by`；Radar/action-feed 仍 creator 范围。
- 前端 `AccessRequestsInbox` 增加 `canReview`；房间 share dialog 用 `canMutateShareLink`，文档库 inbox 仍默认 `canWrite`。

### Slice 8 — Access 策略 oversight 只读 + GET/PUT ACL

- `GET /deal-rooms/:id/access-policy` 需 `NeedView`（oversight 可看；未加入房间的 workspace 成员 403）。
- `PUT` 仍 `NeedManage`；handler 把 `ErrNotRoomAdmin` 映射为 403（不再落成 500）。
- Access 页签对 oversight 可见、只读（无保存、不拉 billing / 申请 inbox）；房间 member/guest 仍不显示该页签。
- `DealRoomAccessControlTab` 不再写死 `canManage = true`。

### Slice 9 — 房间级只读 API 收口 `NeedView`

- `authorizeLinkView`：房间 link 需 `NeedView`（房间成员含 guest + oversight）；文档 link 仍只靠 workspace 成员（RBAC）。
- `ListDealRoomLinks` / `ListDealRoomLinksPage` 先 `requireDealRoomView`：房间不存在 404；无 View 403。未知 UUID 不返回 token 列表。
- `GET /links/:id` 及 access-rules / invitations / access-requests / access-logs / analytics / index-file / file-requests / uploaded-files 走 `loadLinkForView`。未改 `GetByID` / `linkResponse` 签名。
- Ask inbox / FAQ / security-events 仍 `NeedManage`。`GET ask-policy` 补上 `authorizeAskHostOwnerView`（配额与路由策略不是 View 面）。
- 申请 inbox 仍 Slice 7：oversight / 非 manage 空列表，不把 PII 降到 NeedView。

### Slice 10 — link 集成测试可跑 + 只读 ACL 覆盖

- 补回 `stubVisitorAskKnowledge` 与 `Handler.verifyLinkDocumentAccess`（委托 `evaluateLinkDocumentAccess`），link 包 `go test -tags=integration` 重新可编译。
- 默认连库与 dealroom 对齐：先 `dealsignal:dealsignal@127.0.0.1:5435`，再 5436 / `test:test`。fixture 创建人写入 workspace `owner`，文档 link 的 Ask Host / Formal 集成不再误 403。
- 集成覆盖：`ListDealRoomLinks` / `GetRoomAccessPolicy` 的 owner、oversight、未加入成员、房间 guest；oversight 不能建/改房间 link。
- `empty allowlist` 用例显式传 `FolderScopeMode=allowlist`（创建默认已是 full-room）。
- 全量 `./internal/link` 集成仍有既有失败（Ask AI 默认值、quota 路由、formal 到期调度、`conn busy`），不作为本切片绿灯。

### Slice 11 — P2 Members 独立页签

- `DEAL_ROOM_PAGE_TABS` 增加 `members`（`documents` 之后）；能打开房间的人都可见（含 oversight / 房间 guest）。
- 新 `DealRoomMembersTab`：`canManage` 才显示邀请；名单复用 `DealRoomMembersPanel`。邀请默认角色仍是 `guest`，文案不把室 visitor 写成 billed seat。
- Settings 去掉邀请按钮与名单，只留 NDA / 审批 / 状态 / 成员数；成员数行指向 Members 页签。
- `settings` 仍是非水平页签（`?tab=settings`），不进左导航。

### Slice 12 — P3 工作区成员页收口

- 去掉 `actorRole` 在缓存邮箱缺失时回退为 `owner`：识别不出自己则不展示邀请，管理动作失败关闭。
- 工作区角色文案带平面前缀（Workspace owner/admin/member/guest）；数据室 Members 页说明这是室角色，guest 仍叫 Visitor。
- 席位 UsageBar 仍只绑 billing `seatsUsed`/`seatsLimit`；文案写明不计数据室成员与分享链接访客。工作区 guest 不计席。

### Slice 13 — P4 上线闸门测试

- 工作区 guest + 房间 admin 可 `CreateDealRoomLink`；工作区 member + 房间 guest 不可（`ErrNotRoomAdmin`）。
- `ListRooms` 的 workspace 级 Redis 缓存 `dealrooms:list:v2:{wsID}` 仍存全量；`ListRoomsForUser` 在缓存命中后按 `room_members` 过滤，未加入的房间不泄漏。
- 房间 owner 邀请下拉只有 Admin/Member/Visitor，没有 Owner；房间 admin 邀请没有 Admin/Owner。
- 不改 `CreateRoom` 服务层（guest 建室仍靠 HTTP RBAC `write_content`）；不改 analytics `CanEdit`；不全量修 `./internal/link` 集成。

### Slice 14 — 上线闸门收口

- P4：free 满席邀室 member 后 `GetBilling.seatsUsed` 仍为 1。
- 房间详情级巡视横幅（Documents 等页签上方都可见）。
- 邀请下拉：无 `actorRoomRole` 时用空集合，不发明 guest|member；Members 页签同样不展示邀请。
- `ErrNotRoomAdmin` HTTP 码稳定为 `not_room_admin`（dealroom + 房间 link 创建/策略）；`ErrCannotManageMember` 仍是 `cannot_manage_member`；无 View 仍是 `forbidden`。

---

## 下一步

1. **analytics `key_page_settings.CanEdit`** 仍是 workspace `manage_workspace`，本轮不改。
2. 全量 `./internal/link` 集成既有失败不作为本分支回归门。

## 明确不做

- 所有权转让
- 删除房间 / restore / 清空 analytics 扩权
- 文件夹 ACL UI
- 合并 NDA 类型
- 新 `POST /deal-rooms/:id/uploads`

---

## 验证

```bash
cd apps/api
GOTOOLCHAIN=auto go test ./internal/roomacl ./internal/dealroom ./internal/workspace ./internal/link ./internal/knowledge
GOTOOLCHAIN=auto go test -tags=integration ./internal/dealroom ./internal/workspace
GOTOOLCHAIN=auto go test -tags=integration ./internal/link -run 'TestListDealRoomLinks_NeedView_Integration|TestGetRoomAccessPolicy_NeedView_Integration|TestCreateDealRoomLink_OversightForbidden_Integration|TestAuthorizeLinkMutate_OversightForbidden_Integration|TestCreateDealRoomLink_WorkspaceGuestRoomAdminAllowed_Integration|TestCreateDealRoomLink_WorkspaceMemberRoomGuestForbidden_Integration|TestVerifyLinkDocumentAccess_DealRoomScope|TestDealRoomScope_CreateWithEmptyScopeMeansDenyAll|TestDocumentsForAccessResponse_DealRoomScope'

cd ../web
pnpm exec vitest run --coverage.enabled=false \
  src/lib/dealRoomCapabilities.test.ts \
  src/components/deal-rooms/DealRoomFolderTree.test.tsx \
  src/components/deal-rooms/DealRoomQATab.test.tsx \
  src/components/links/share/AnalyticsTab.test.tsx \
  src/components/links/share/ManagementTab.test.tsx \
  src/components/deal-rooms/DealRoomMembersPanel.test.tsx \
  src/components/deal-rooms/DealRoomMembersTab.test.tsx \
  src/components/deal-rooms/InviteMemberDialog.test.tsx \
  src/components/deal-rooms/DealRoomNavBPrime.test.tsx \
  src/components/deal-rooms/DealRoomAccessRequestsPanel.test.tsx \
  src/components/deal-rooms/DealRoomAccessControlTab.test.tsx \
  src/hooks/useDealRoomTab.test.ts \
  src/routes/deal-rooms/detail.test.tsx \
  src/routes/settings/members.test.tsx
```

`go test -tags=integration ./internal/link` 现可编译；ACL 相关 `-run` 子集为绿灯。全量 link 集成仍有既有失败，不作为本分支回归门。
