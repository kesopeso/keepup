-- Never silently turn private account routes into anonymously accessible legacy routes.
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM routes WHERE owner_user_id IS NOT NULL)
       OR EXISTS(SELECT 1 FROM route_members WHERE user_id IS NOT NULL) THEN
        RAISE EXCEPTION 'handle account-owned route data before reverting migration 0006';
    END IF;
END $$;
DROP TABLE auth_socket_tickets;
ALTER TABLE route_members DROP CONSTRAINT route_members_account_left_check;
ALTER TABLE route_members DROP COLUMN user_id;
ALTER TABLE routes DROP COLUMN owner_user_id;
