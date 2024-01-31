#!/bin/bash
set -o nounset -o pipefail -o errexit

DB_TYPE=${1-sqlite}

if ! command -v newman &> /dev/null; then
    echo "Newman could not be found. Run 'npm install -g newman' first."
    exit 1
fi

for i in "$@"; do
    case $i in
        --migration)
            MIGRATION=1
            ;;
    esac
done

script_path=$(realpath "${BASH_SOURCE[0]}")
script_dir=$(dirname "$script_path")

echo "Compiling."
(cd "$script_dir/.." || exit 1; CGO_ENABLED=0 go build)

cd "$script_dir" || exit 1

# Download previous release (when upgrade testing)
if [ "${MIGRATION-0}" -eq 1 ]; then
    if [ ! -f wakapi_linux_amd64.zip ]; then
        echo "Downloading latest release"
        curl https://github.com/muety/wakapi/releases/latest/download/wakapi_linux_amd64.zip -O -L
    fi
    unzip -o wakapi_linux_amd64.zip
    echo "Running tests with release version"
fi

cleanup() {
    if [ -n "$pid" ] && ps -p "$pid" > /dev/null; then
        kill -TERM "$pid"
    fi
    if [ "${docker_down-0}" -eq 1 ]; then
        docker compose -f "$script_dir/compose.yml" down
    fi
}
trap cleanup EXIT

# Initialise test data
case $DB_TYPE in
    postgres|mysql|mariadb|cockroach|mssql)
    docker compose -f "$script_dir/compose.yml" down

    docker_down=1
    docker compose -f "$script_dir/compose.yml" up --wait --detach "$DB_TYPE"

    config="config.$DB_TYPE.yml"
    if [ "$DB_TYPE" == "mariadb" ]; then
        config="config.mysql.yml"
    fi

    db_port=0
    if [ "$DB_TYPE" == "postgres" ]; then
        db_port=55432
    elif [ "$DB_TYPE" == "cockroach" ]; then
        db_port=56257
    elif [ "$DB_TYPE" == "mssql" ]; then
        db_port=1433
    else
        db_port=26257
    fi

    for _ in $(seq 0 30); do
        if netstat -tulpn 2>/dev/null | grep "LISTEN" | tr -s ' ' | cut -d' ' -f4 | grep -E ":$db_port$" > /dev/null; then
            break
        fi
        sleep 1
    done
    ;;

    sqlite|*)
    config="config.sqlite.yml"
    rm -f wakapi_testing.db
    ;;
esac

wait_for_wakapi () {
    counter=0
    echo "Waiting for Wakapi to come up ..."
    until curl --output /dev/null --silent --get --fail http://localhost:3000/api/health; do
        if [ "$counter" -ge 30 ]; then
            echo "Waited for 30s, but Wakapi failed to come up ..."
            exit 1
        fi

        printf '.'
        sleep 1
        counter=$((counter+1))
    done
    sleep 1
    printf "\n"
}

start_wakapi_background() {
    path=$1
    config=$2

    "$path" -config "$config" &
    pid=$!
    wait_for_wakapi
}

kill_wakapi() {
    echo "Shutting down Wakapi ..."
    kill -TERM $pid
}

# Need to create database for mssql
if [ "$DB_TYPE" == "mssql" ]; then
    echo "Sleep for 5s to wait for db to be ready..."
    sleep 5
    echo "Creating database in mssql..."
    docker compose -f "$script_dir/compose.yml" exec mssql \
        /opt/mssql-tools/bin/sqlcmd -S localhost -U SA -P 'Hard!password123' -Q 'CREATE DATABASE wakapi'
fi

# Run original wakapi
echo "Configuration file: $config"
if [ "${MIGRATION-0}" -eq 1 ]; then
    echo "Running last release ..."
    start_wakapi_background "./wakapi" "$config"
    kill_wakapi
fi

echo "Running current build ..."
start_wakapi_background "../wakapi" "$config"
kill_wakapi

# Only sqlite has data

if [ "$DB_TYPE" == "sqlite" ]; then
    echo "Creating database and schema ..."
    sqlite3 wakapi_testing.db < schema.sql
    echo "Importing seed data ..."
    sqlite3 wakapi_testing.db < data.sql

    start_wakapi_background "../wakapi" "$config"
    echo "Running test collection ..."
    if ! newman run "wakapi_api_tests.postman_collection.json"; then
        exit_code=$?
        echo "newman failed with exit code $exit_code"
        exit "$exit_code"
    fi
    kill_wakapi
fi

