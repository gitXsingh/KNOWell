ALTER TABLE repositories
ADD COLUMN webhook_secret text NOT NULL DEFAULT '';
