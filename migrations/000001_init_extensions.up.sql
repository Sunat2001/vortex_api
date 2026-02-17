-- UUID v7 extension (optional - will use gen_random_uuid() if not available)
-- To install: https://github.com/fboulnois/pg_uuidv7

DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_uuidv7;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_uuidv7 extension not available, will use gen_random_uuid() instead';
END $$;