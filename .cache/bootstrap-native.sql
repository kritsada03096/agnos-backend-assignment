\set ON_ERROR_STOP on
\getenv app_user AGNOS_DB_USER
\getenv app_password AGNOS_DB_PASSWORD
\getenv app_db AGNOS_DB_NAME
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L', :'app_user', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_user')
\gexec
SELECT format('CREATE DATABASE %I OWNER %I ENCODING ''UTF8'' TEMPLATE template0', :'app_db', :'app_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'app_db')
\gexec
ALTER SYSTEM SET listen_addresses = 'localhost';
