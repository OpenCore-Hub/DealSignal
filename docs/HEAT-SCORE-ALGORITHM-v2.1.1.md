# DealSignal v2.1.1 热度评分算法文档

> MVP 规则版热度评分与关键页识别
> 日期：2026-06-20
> 状态：已批准

---

## 1. 设计目标

热度评分（Heat Score）是 DealSignal 的核心差异化能力，目标：
1. **量化意图**：把分散的阅读行为转化为 0-100 的单一分数
2. **跨圈层可比**：创始人、IR、销售使用同一底层计算，但权重不同
3. **可解释**：用户能理解分数为什么高/低
4. **可行动**：分数变化直接触发跟进建议

---

## 2. 核心公式

### 2.1 基础评分公式

```typescript
function calculateHeatScore(
  events: ReaderEvent[],
  config: HeatScoreConfig
): number {
  const rawScore =
    config.weights.opens * countOpens(events) +
    config.weights.revisits * countRevisits(events) +
    config.weights.avgDurationMinutes * getAvgDurationMinutes(events) +
    config.weights.keyPageViews * countKeyPageViews(events, config.keyPages) +
    config.weights.forwardSignals * countForwardSignals(events) +
    config.weights.downloads * countDownloads(events) -
    config.weights.bouncePenalty * countBounces(events);

  return Math.min(100, Math.max(0, Math.round(rawScore)));
}
```

### 2.2 默认权重配置

#### 融资创始人场景（Founder）

```json
{
  "name": "founder",
  "weights": {
    "opens": 3,
    "revisits": 18,
    "avgDurationMinutes": 12,
    "keyPageViews": 25,
    "forwardSignals": 15,
    "downloads": 8,
    "bouncePenalty": 10
  },
  "keyPages": ["financials", "team", "traction", "market"],
  "thresholds": {
    "hot": 75,
    "warm": 40,
    "cold": 0
  }
}
```

#### 投资机构 IR 场景（Investor IR）

```json
{
  "name": "investor_ir",
  "weights": {
    "opens": 2,
    "revisits": 12,
    "avgDurationMinutes": 10,
    "keyPageViews": 20,
    "forwardSignals": 8,
    "downloads": 5,
    "bouncePenalty": 10
  },
  "keyPages": ["performance", "distribution", "strategy", "portfolio"],
  "thresholds": {
    "hot": 70,
    "warm": 35,
    "cold": 0
  }
}
```

#### B2B 销售场景（Sales）

```json
{
  "name": "sales",
  "weights": {
    "opens": 2,
    "revisits": 15,
    "avgDurationMinutes": 10,
    "keyPageViews": 28,
    "forwardSignals": 20,
    "downloads": 5,
    "bouncePenalty": 12
  },
  "keyPages": ["pricing", "security", "case_studies", "implementation"],
  "thresholds": {
    "hot": 72,
    "warm": 38,
    "cold": 0
  }
}
```

---

## 3. 事件定义

### 3.1 事件类型

| 事件 | 定义 | 去重规则 |
|---|---|---|
| `open` | 首次打开链接 | 每会话 1 次，会话间隔 ≥ 30 分钟 |
| `revisit` | 同一会话或新会话再次打开 | 距上次 open ≥ 1 小时 |
| `page_view` | 查看某页 ≥ 2 秒 | 同页 5 分钟内只计 1 次 |
| `key_page_view` | 查看关键页 ≥ 3 秒 | 同页 10 分钟内只计 1 次 |
| `forward_signal` | 同一链接出现新邮箱/设备/IP | 每个新访问者计 1 次 |
| `download` | 点击下载按钮并成功 | 每次下载计 1 次 |
| `bounce` | 打开后 5 秒内离开且只看 1 页 | 每次会话最多 1 次 |

### 3.2 会话定义

- 默认会话超时：30 分钟
- 新设备/新浏览器 = 新会话
- 夜间断档（00:00-06:00）自动开启新会话

---

## 4. 关键页识别

### 4.1 识别方法

MVP 采用**规则 + 关键词**方式：

```typescript
const keyPageRules: Record<string, string[]> = {
  founder: {
    financials: ["financial", "revenue", "projection", "unit economics", "burn", "runway"],
    team: ["team", "founder", "advisor", "hiring"],
    traction: ["traction", "growth", "metric", "mrr", "arr", "customer"],
    market: ["market", "tam", "sam", "som", "opportunity"]
  },
  investor_ir: {
    performance: ["performance", "return", "irr", "multiple", "nav"],
    distribution: ["distribution", "dpi", "rvpi", "tvpi", "capital"],
    strategy: ["strategy", "thesis", "allocation", "outlook"],
    portfolio: ["portfolio", "company", "investment"]
  },
  sales: {
    pricing: ["pricing", "price", "cost", "fee", "quote", "proposal"],
    security: ["security", "compliance", "soc2", "gdpr", "encryption"],
    case_studies: ["case study", "customer story", "testimonial", "roi"],
    implementation: ["implementation", "onboarding", "deployment", "timeline"]
  }
};
```

### 4.2 识别逻辑

1. 提取页面文本（前 500 字）
2. 转小写，去除标点
3. 匹配关键词列表
4. 取匹配度最高的类别
5. 匹配度 ≥ 0.3 视为该关键页
6. 一页可能属于多个类别（取最高匹配）

### 4.3 _fallback_

如果页面未匹配任何关键页：
- 记录为 `general`
- 不纳入 key_page_views 计算
- 但仍计入 avgDuration

---

## 5. 热度分层

### 5.1 分层规则

| 层级 | 分数范围 | 颜色 | 跟进建议 |
|---|---|---|---|
| Hot | ≥ threshold.hot | Red | 24 小时内跟进 |
| Warm | threshold.warm ~ threshold.hot | Amber | 48 小时内跟进 |
| Cold | < threshold.warm | Blue | 加入培养序列 |

### 5.2 趋势计算

```typescript
function calculateTrend(
  currentScore: number,
  previousScore: number
): "rising" | "stable" | "falling" {
  const delta = currentScore - previousScore;
  if (delta >= 10) return "rising";
  if (delta <= -10) return "falling";
  return "stable";
}
```

---

## 6. 分数解释

### 6.1 分数构成可视化

在 Insights 页面展示：
- 总分
- 各维度贡献条形图
- 与同类文档的平均分对比

### 6.2 解释文案

| 场景 | 文案示例 |
|---|---|
| Hot + 财务页反复查看 | "Sarah 对财务模型表现出强烈兴趣，建议主动提供假设说明。" |
| Warm + 团队页停留 | "Marcus 正在评估团队背景，可补充创始人经历资料。" |
| Cold + 短停留 | "Wei 仅快速浏览，建议 1 周后发送进展更新。" |
| Hot + 多人转发 | "Acme Corp 内部已转发给 4 人，建议联系 champion 了解决策链。" |

---

## 7. 信号生成规则

### 7.1 高意图信号

当满足以下条件时生成 Hot Signal：
- 单人会话中查看 ≥ 3 个关键页
- 关键页总停留 ≥ 2 分钟
- 24 小时内回访 ≥ 2 次
- 同一链接被转发至 ≥ 3 个新访问者

### 7.2 风险信号

- 同一链接 1 小时内被 ≥ 5 个不同地区访问
- 高敏感文档被下载
- 链接过期后仍有访问尝试

---

## 8. 演进路线

### 8.1 MVP（v2.1.1）

- 规则加权评分
- 关键词关键页识别
- 静态权重配置

### 8.2 Phase 2（v2.2）

- 引入时间衰减（近期行为权重更高）
- A/B 测试不同权重
- 用户可微调权重

### 8.3 Phase 3（v2.3）

- 机器学习模型校准
- 基于历史成交数据训练
- 自动识别关键页（无需关键词）

---

## 9. 实现接口

### 9.1 类型定义

```typescript
interface HeatScoreConfig {
  name: "founder" | "investor_ir" | "sales";
  weights: {
    opens: number;
    revisits: number;
    avgDurationMinutes: number;
    keyPageViews: number;
    forwardSignals: number;
    downloads: number;
    bouncePenalty: number;
  };
  keyPages: Record<string, string[]>;
  thresholds: {
    hot: number;
    warm: number;
    cold: number;
  };
}

interface HeatScoreResult {
  score: number;
  level: "hot" | "warm" | "cold";
  trend: "rising" | "stable" | "falling";
  breakdown: Record<string, number>;
  topKeyPages: string[];
}
```

### 9.2 计算函数

```typescript
function calculateHeatScore(
  events: ReaderEvent[],
  config: HeatScoreConfig,
  previousScore?: number
): HeatScoreResult;
```

---

## 10. 测试用例

### 10.1 用例 1：高意图创始人

事件：
- open 1 次
- revisit 2 次
- avg duration 5 分钟
- key page views: financials×2, team×1
- forward 0
- download 0
- bounce 0

计算（founder 权重）：
```
3×1 + 18×2 + 12×5 + 25×3 + 15×0 + 8×0 - 10×0
= 3 + 36 + 60 + 75 = 174 → 100 (cap)
```
结果：Hot

### 10.2 用例 2：低意图浏览

事件：
- open 1
- revisit 0
- avg duration 0.5 分钟
- key page views 0
- forward 0
- download 0
- bounce 1

计算：
```
3×1 + 0 + 12×0.5 + 0 + 0 + 0 - 10×1
= 3 + 6 - 10 = -1 → 0
```
结果：Cold

---

## 11. 版本历史

| 版本 | 日期 | 变更 |
|---|---|---|
| v2.1.1 | 2026-06-20 | MVP 规则版热度评分算法 |
