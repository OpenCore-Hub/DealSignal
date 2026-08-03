-- Structure locks for deal-room documents (folders lock via settings.folders JSON).
ALTER TABLE deal_room_documents
    ADD COLUMN IF NOT EXISTS locked BOOLEAN NOT NULL DEFAULT false;
