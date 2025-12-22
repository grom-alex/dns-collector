-- Rollback performance tuning settings
-- PostgreSQL migration script
-- Version: 1.0.0

-- Reset database settings to defaults
ALTER DATABASE dns_collector RESET statement_timeout;
ALTER DATABASE dns_collector RESET lock_timeout;
ALTER DATABASE dns_collector RESET idle_in_transaction_session_timeout;
ALTER DATABASE dns_collector RESET work_mem;
ALTER DATABASE dns_collector RESET maintenance_work_mem;
ALTER DATABASE dns_collector RESET jit;
ALTER DATABASE dns_collector RESET effective_cache_size;
ALTER DATABASE dns_collector RESET random_page_cost;
