-- Run against the configured primary PostgreSQL database, using the application's
-- search_path. This script is read-only and does not print credentials or keys.
BEGIN TRANSACTION READ ONLY;

SELECT current_database() AS database_name, current_schema() AS schema_name;

WITH required(column_name) AS (
  VALUES ('quota'), ('used_quota'), ('aff_quota'), ('aff_history')
)
SELECT r.column_name, format_type(a.atttypid, a.atttypmod) AS actual_type,
       CASE WHEN a.atttypid = 'bigint'::regtype THEN 'PASS'
            WHEN a.attname IS NULL THEN 'MISSING'
            ELSE 'MIGRATION_REQUIRED' END AS result
FROM required r
LEFT JOIN pg_attribute a ON a.attrelid = to_regclass('users')
  AND a.attname = r.column_name AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY r.column_name;

SELECT count(*) AS user_count, sum(quota::numeric) AS wallet_quota,
       sum(used_quota::numeric) AS used_quota,
       sum(aff_quota::numeric) AS affiliate_quota,
       sum(aff_history::numeric) AS affiliate_history
FROM users;

SELECT type, count(*) AS channel_count FROM channels GROUP BY type ORDER BY type;
SELECT count(*) AS channel_count, count(proxy_id) AS bound_channel_count FROM channels;
SELECT status, count(*) AS proxy_count FROM proxies GROUP BY status ORDER BY status;
SELECT status, count(*) AS order_count, sum(amount::numeric) AS amount,
       sum(money::numeric) AS money FROM top_ups GROUP BY status ORDER BY status;

ROLLBACK;
