ALTER TABLE deal_rooms
    ADD CONSTRAINT uk_deal_rooms_slug UNIQUE (slug);

CREATE OR REPLACE FUNCTION prevent_access_request_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status != 'pending' THEN
        RAISE EXCEPTION 'access request is already finalized and cannot be modified';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS room_access_requests_prevent_update ON room_access_requests;
CREATE TRIGGER room_access_requests_prevent_update
    BEFORE UPDATE ON room_access_requests
    FOR EACH ROW EXECUTE FUNCTION prevent_access_request_mutation();
