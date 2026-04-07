-- +goose Up
-- Admin panel uses uuid.Nil as banned_by/muted_by (system admin has no row in users table).
-- Drop the FK constraints so uuid.Nil is accepted without a referential integrity error.
ALTER TABLE bans  DROP CONSTRAINT IF EXISTS bans_banned_by_fkey;
ALTER TABLE mutes DROP CONSTRAINT IF EXISTS mutes_muted_by_fkey;

-- +goose Down
ALTER TABLE bans  ADD CONSTRAINT bans_banned_by_fkey  FOREIGN KEY (banned_by)  REFERENCES users(id);
ALTER TABLE mutes ADD CONSTRAINT mutes_muted_by_fkey FOREIGN KEY (muted_by) REFERENCES users(id);
