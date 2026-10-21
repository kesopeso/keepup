DROP TABLE account_sharing_leases;
DROP TABLE account_live_connections;
ALTER TABLE auth_sessions DROP COLUMN revoked_reason;
ALTER TABLE route_members DROP COLUMN live_absent_since;
