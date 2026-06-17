# 项目脚手架与数据库 Schema 落地

## Description
搭建 DealSignal 项目基础结构，落地 PostgreSQL 数据库 schema、迁移机制、基础配置，使后续后端功能可以基于统一的数据模型开发。

## Source
database-model.md Section 4, sql/schema.sql

## Hard Constraints
- 所有多租户表必须按 workspace_id 隔离

## Acceptance Criteria
- [x] PostgreSQL schema 通过初始化脚本可正常创建所有 P0 表与索引
- [x] 迁移工具配置完成并可在新环境一键应用
- [x] 项目目录结构清晰区分 backend、frontend、shared 等模块
- [x] README 包含本地启动数据库与运行迁移的命令
- [x] TypeScript / lint / build 基础检查通过

## Validation
- [x] 运行迁移命令后数据库包含 users、workspaces、documents、document_versions、smart_links 等核心表
- [x] CI 或本地脚本执行 lint 与 build 不报错

## Dependencies
无

## Type
infra

## Priority
high

## Risk Class
build_failure

## PRD Reference
PRD.md Section 7, database-model.md, sql/schema.sql

## SPEC Reference

- SPEC file: `tasks/spec-dealsignal-v1.md`
- Relevant sections: SPEC Section 2.4 File Structure, 3.1 Schema Changes, 3.4 Migration Plan
- API endpoints: N/A — infrastructure
- Data model: All P0 tables from sql/schema.sql; add document_page_tiles table
- Testing guidance: Migration applies cleanly; lint/build passes
