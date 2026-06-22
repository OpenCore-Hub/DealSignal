DROP TRIGGER IF EXISTS room_access_requests_prevent_update ON room_access_requests;
DROP FUNCTION IF EXISTS prevent_access_request_mutation();

ALTER TABLE deal_rooms
    DROP CONSTRAINT IF EXISTS uk_deal_rooms_slug;
