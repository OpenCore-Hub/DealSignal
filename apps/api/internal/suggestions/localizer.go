package suggestions

import "strings"

// localizedStrings holds user-facing copy for suggestion generation.
type localizedStrings struct {
	hotSignalTitle      string
	riskAlertTitle      string
	followUpTitle       string
	hotSignalReasonTmpl string
	hotSignalAction     string
	downloadReasonTmpl  string
	downloadAction      string
	revisitReasonTmpl   string
	revisitAction       string
	riskReasonTmpl      string
	riskAction          string
}

func newLocalizedStrings(lang string) *localizedStrings {
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return &localizedStrings{
			hotSignalTitle:      "高意向信号",
			riskAlertTitle:      "风险预警",
			followUpTitle:       "跟进建议",
			hotSignalReasonTmpl: "热度评分达到 %d（%s），该联系人在 %d 次打开中查看了 %d 个关键页面",
			hotSignalAction:     "立即发送 follow-up 邮件并提供深度资料",
			downloadReasonTmpl:  "联系人在最近 %d 次访问中尝试了下载",
			downloadAction:      "确认对方是否收到文件并询问反馈",
			revisitReasonTmpl:   "联系人重复访问了 %d 次，表现出持续兴趣",
			revisitAction:       "发送针对性的内容或安排一次通话",
			riskReasonTmpl:      "%d 次访问后快速离开，平均停留 %.1f 分钟",
			riskAction:          "优化材料首屏或换一种触达方式",
		}
	}
	return &localizedStrings{
		hotSignalTitle:      "High-intent signal",
		riskAlertTitle:      "Risk alert",
		followUpTitle:       "Follow-up suggestion",
		hotSignalReasonTmpl: "Heat score reached %d (%s); the contact viewed %d key pages across %d opens",
		hotSignalAction:     "Send a follow-up email with in-depth materials now",
		downloadReasonTmpl:  "Contact attempted a download in the last %d visits",
		downloadAction:      "Confirm whether they received the file and ask for feedback",
		revisitReasonTmpl:   "Contact revisited %d times, showing continued interest",
		revisitAction:       "Send targeted content or schedule a call",
		riskReasonTmpl:      "%d visits left quickly, avg stay %.1f min",
		riskAction:          "Optimize the first screen of the material or try another outreach method",
	}
}
