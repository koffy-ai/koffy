#!/bin/sh
set -eu

# This script runs only during first-time initialization of the local MySQL
# data directory. The official MySQL entrypoint provides mysql_socket and the
# MYSQL_* environment variables.
case "${MYSQL_USER}" in
  *"'"*)
    echo "MYSQL_USER must not contain a single quote." >&2
    exit 1
    ;;
esac

mysql --protocol=socket -uroot -p"${MYSQL_ROOT_PASSWORD}" <<SQL
CREATE DATABASE IF NOT EXISTS casdoor CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE DATABASE IF NOT EXISTS koffy CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
GRANT ALL PRIVILEGES ON casdoor.* TO '${MYSQL_USER}'@'%';
GRANT ALL PRIVILEGES ON koffy.* TO '${MYSQL_USER}'@'%';
FLUSH PRIVILEGES;
SQL
