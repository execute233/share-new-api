-- 在应用配置的 PostgreSQL 主数据库中执行，并使用应用的 search_path。
-- 本脚本仅执行只读查询，不输出凭据或密钥，不执行迁移。
-- 默认表名适用于项目现有 GORM 命名方式；自定义命名时需相应调整。
BEGIN TRANSACTION READ ONLY;

SELECT current_database() AS database_name, current_schema() AS schema_name;

-- 四个额度字段必须均为 BIGINT。
-- PASS：通过；MISSING：字段缺失；MIGRATION_REQUIRED：需要确认并迁移类型。
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

-- 用户数量与额度汇总；升级前后对比，并结合逐行快照检查数据完整性。
SELECT count(*) AS user_count, sum(quota::numeric) AS wallet_quota,
       sum(used_quota::numeric) AS used_quota,
       sum(aff_quota::numeric) AS affiliate_quota,
       sum(aff_history::numeric) AS affiliate_history
FROM users;

-- 渠道类型、代理绑定及代理状态统计，不查询代理凭据。
SELECT type, count(*) AS channel_count FROM channels GROUP BY type ORDER BY type;
SELECT count(*) AS channel_count, count(proxy_id) AS bound_channel_count FROM channels;
SELECT status, count(*) AS proxy_count FROM proxies GROUP BY status ORDER BY status;

-- 订单状态、充值数量与金额汇总。
SELECT status, count(*) AS order_count, sum(amount::numeric) AS amount,
       sum(money::numeric) AS money FROM top_ups GROUP BY status ORDER BY status;

ROLLBACK;
