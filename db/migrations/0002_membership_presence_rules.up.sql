DROP INDEX IF EXISTS route_members_route_alias_unique_idx;

CREATE UNIQUE INDEX route_members_route_alias_unique_idx
    ON route_members (route_id, LOWER(display_name))
    WHERE status <> 'left';

ALTER TABLE path_segments
    DROP CONSTRAINT IF EXISTS path_segments_end_reason_check;

ALTER TABLE path_segments
    ADD CONSTRAINT path_segments_end_reason_check
    CHECK (end_reason IN ('stopped', 'disconnected', 'left', 'route_closed'));
