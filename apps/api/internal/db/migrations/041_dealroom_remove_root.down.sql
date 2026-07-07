-- Revert the root folder removal migration.
-- Documents and permissions are moved back to root, and the root folder row
-- is restored at the beginning of each room's settings.folders array.

-- Move documents from /general back to root.
UPDATE deal_room_documents
SET folder_path = '/'
WHERE folder_path = '/general';

-- Move permissions from /general back to root (empty folder_path).
UPDATE room_member_folder_permissions
SET folder_path = ''
WHERE folder_path = '/general';

-- Restore the root folder object at the front of each room's settings.folders array.
UPDATE deal_rooms
SET settings = jsonb_set(
    settings,
    '{folders}',
    jsonb_build_array(
        jsonb_build_object('path', '/', 'name', 'Root', 'sort_order', 0)
    ) || COALESCE(settings->'folders', '[]'::jsonb)
)
WHERE settings ? 'folders';
