-- 000003_add_addon_group_name.sql
-- Add grouping support for addons (toppings, jalea, extras, etc.)

ALTER TABLE addons
ADD COLUMN IF NOT EXISTS group_name VARCHAR(50) NOT NULL DEFAULT 'extras';

CREATE INDEX IF NOT EXISTS idx_addons_group_name ON addons (group_name);
CREATE INDEX IF NOT EXISTS idx_addons_group_active_order ON addons (group_name, is_active, display_order);

UPDATE addons
SET group_name = 'extras'
WHERE group_name IS NULL OR btrim(group_name) = '';
