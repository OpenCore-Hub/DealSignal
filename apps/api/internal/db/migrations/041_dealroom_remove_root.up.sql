-- Remove the legacy root folder concept from deal rooms.
-- Root documents and permissions are moved to /general, and the root folder
-- row is removed from each room's settings JSON.

-- Move documents previously stored at root into the general folder.
UPDATE deal_room_documents
SET folder_path = '/general'
WHERE folder_path = '/';

-- Move root-level permissions (empty folder_path) to the general folder.
UPDATE room_member_folder_permissions
SET folder_path = '/general'
WHERE folder_path = '';

-- Remove the root folder object from each room's settings.folders array.
UPDATE deal_rooms
SET settings = jsonb_set(
    settings,
    '{folders}',
    COALESCE(
        (
            SELECT jsonb_agg(f)
            FROM jsonb_array_elements(settings->'folders') AS f
            WHERE f->>'path' != '/'
        ),
        '[]'::jsonb
    )
)
WHERE settings ? 'folders';

-- Ensure every room has at least a general folder if folders became empty.
UPDATE deal_rooms
SET settings = jsonb_set(
    settings,
    '{folders}',
    '[{"path": "/general", "name": "General", "sort_order": 0}]'::jsonb
)
WHERE settings ? 'folders'
  AND jsonb_array_length(settings->'folders') = 0;
