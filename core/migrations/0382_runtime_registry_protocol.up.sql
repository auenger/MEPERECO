-- Runtime registry validates registered protocol IDs at the application boundary.
-- Keep a stable format invariant in storage instead of enumerating providers.
ALTER TABLE runtime_profile DROP CONSTRAINT runtime_profile_protocol_family_check;
ALTER TABLE runtime_profile ADD CONSTRAINT runtime_profile_protocol_family_check CHECK (protocol_family ~ '^[a-z][a-z0-9_-]{0,63}$') NOT VALID;
