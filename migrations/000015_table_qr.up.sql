ALTER TABLE venue_tables ADD COLUMN qr_token_encrypted text;
CREATE UNIQUE INDEX venue_tables_qr_hash_idx ON venue_tables (qr_token_hash);
