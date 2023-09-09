#!/bin/bash -e
cd "$(dirname "$0")"

DB_HOST=${DB_HOST:-127.0.0.1}
DB_PORT=${DB_PORT:-3306}
DB_USER=${DB_USER:-isucon}
DB_PASS=${DB_PASS:-isucon}
DB_NAME=${DB_NAME:-isulibrary}

set -x
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < 0_schema.sql
mysql <<< 'ALTER INSTANCE DISABLE INNODB REDO_LOG; SET GLOBAL slow_query_log = 0;'
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < 1_data.sql &
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < 2_book_title_suffix.sql &
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < 2_book_author_suffix.sql &
wait
mysql <<< 'ALTER INSTANCE ENABLE INNODB REDO_LOG; SET GLOBAL slow_query_log = 1;'
