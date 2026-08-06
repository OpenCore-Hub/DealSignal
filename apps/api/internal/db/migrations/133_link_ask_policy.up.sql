-- Phase B: visitor Ask policy columns on links (AI lane gating; default host-only).
ALTER TABLE links
    ADD COLUMN ask_mode TEXT NOT NULL DEFAULT 'supervised'
        CHECK (ask_mode IN ('self_serve', 'supervised', 'formal')),
    ADD COLUMN ask_ai_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN ask_ai_monthly_quota INT;

COMMENT ON COLUMN links.ask_mode IS 'Visitor Ask routing mode: self_serve | supervised | formal';
COMMENT ON COLUMN links.ask_ai_enabled IS 'When true, Pro+ links may use AI lane (Phase B); Standard remains host-only';
COMMENT ON COLUMN links.ask_ai_monthly_quota IS 'Optional per-link AI Ask monthly cap; NULL uses workspace/plan default';
