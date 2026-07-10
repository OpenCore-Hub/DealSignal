-- Migration: add intent classification tag to visitor questions.
ALTER TABLE link_visitor_questions
    ADD COLUMN IF NOT EXISTS intent_tag TEXT NOT NULL DEFAULT '';
