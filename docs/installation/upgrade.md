# Geocube Upgrade
## PostgreSQL database
After upgrading the Geocube server, the database schema might need an update. Apply incrementally each `interface/database/pg/update_X.X.X.sql` with X.X.X corresponding to a Geocube Server version from your previous version to the current version.

```bash
$ psql -h <database_host> -d <database_name> -f interface/database/pg/update_X.X.X.sql
```
