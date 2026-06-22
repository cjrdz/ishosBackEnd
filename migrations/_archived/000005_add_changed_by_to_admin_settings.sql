-- Add changed_by column to admin_settings for auditing
ALTER TABLE admin_settings ADD COLUMN changed_by UUID;

-- Backfill existing rows with the zero UUID so subsequent migrations
-- that enforce NOT NULL won't fail due to existing NULL values.
UPDATE admin_settings SET changed_by = '00000000-0000-0000-0000-000000000000' WHERE changed_by IS NULL;

-- If you intend this column to be NOT NULL in the final schema, uncomment the
-- following line. It is left commented so this migration is safe to run even
-- if your code currently expects the column to be nullable.
-- ALTER TABLE admin_settings ALTER COLUMN changed_by SET NOT NULL;

-- Optionally, add a foreign key constraint if you have an employees/admins table:
-- ALTER TABLE admin_settings ADD CONSTRAINT fk_changed_by FOREIGN KEY (changed_by) REFERENCES employees(id);
