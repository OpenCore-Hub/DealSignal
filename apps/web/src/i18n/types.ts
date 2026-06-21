export type Namespace =
  | "common"
  | "layout"
  | "dashboard"
  | "documents"
  | "links"
  | "contacts"
  | "insights"
  | "settings"
  | "dealRooms"
  | "ai"
  | "formatters";

// 迁移期间先放宽 key 类型检查，待所有文案 key 稳定后可恢复严格类型
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "common";
  }
}
