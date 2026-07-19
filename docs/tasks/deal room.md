## 一、数据室级导航（Deal Room Nav，6 项）

进入某个数据室后，左侧导航切换为当前数据室工具。顶部面包屑显示：

> **Acme Capital / 数据室 / 2026 融资尽调**

| 导航项 | 英文名 | 说明 |
|--------|--------|------|
| **文档** | Documents | 文件夹、文件、权限、版本、水印、批量上传 |
| **访客** | Participants | 受邀用户、角色、访问权限、审批队列 |
| **问答** | Q&A | 尽职调查问答，按主题/文件分组，支持指派、回复、审批 |
| **活动** | Activity | 审计日志：谁看了什么、下载、打印、访问时间 |
| **分析** | Analytics | 该数据室的访问热力、停留时长、高意向行为 |
| **设置** | Room Settings | 该数据室的 NDA、水印、有效期、品牌、通知、安全策略 |

---

## 二、数据室级

```
Deal Room
├── Documents
│   ├── Files & Folders
│   ├── Upload
│   ├── File Requests
│   ├── Permissions
│   └── Index Builder
├── Participants
│   ├── Guests
│   ├── Groups
│   ├── Pending Invitations
│   └── Access Requests
├── Q&A
│   ├── All Questions
│   ├── My Questions
│   ├── By Topic
│   └── By Document
├── Activity
│   ├── View Log
│   ├── Download Log
│   └── Print Log
├── Analytics
│   ├── Summary
│   ├── Visitor Timeline
│   ├── Document Heatmap
│   └── Engagement Score
└── Room Settings
    ├── General
    ├── Security (NDA, watermark, password)
    ├── Notifications
    ├── Branding
    └── Archive / Delete
```

---

## 三、关键设计决策

### 1. 为什么“文档库”不应在顶层？
在纯 VDR 中，文档不是独立目的地，而是服务于某个交易或尽调对象。

### 2. 为什么“分享”是动作而不是导航？
“分享”没有固定内容。用户真正想做的是：
- 在数据室里邀请访客
- 对某个文档生成安全链接
- 设置链接权限

因此“分享”应该表现为：
- 数据室首页的 **“邀请访客”** 主按钮
- 文档行的 **“生成分享链接”** 操作
- 联系人详情页的 **“发送访问邀请”**

### 3. “协议”去哪了？
“协议/Agreements”在 VDR 中通常指 NDA、保密协议、签署流程。建议：
- 如果作为数据室访问门槛：在 **数据室设置 → Security → NDA** 中配置
- 不放在顶层

### 4. “洞察”为什么保留在顶层？
因为决策者需要跨数据室查看整体活动。但进入具体数据室后，应有一个对应的 **Analytics** 二级页，提供项目级洞察。

---

## 四、与现有 DealSignal 的迁移对照

| 现有导航 | 新架构位置 |
|----------|------------|
| 交易雷达 | 首页 / Dashboard |
| 文档库 | 数据室 → 文档  |
| 分享 | 数据室 → 动作按钮 |
| 数据室 | 数据室（核心入口） |
| 联系人 | 联系人 |
| 洞察 | 洞察（跨项目）+ 数据室 → 分析 |
| 协议 | 数据室设置 → Security |
| 设置 | 管理 -> 底部工作区 |

---