package migrations

import (
	"github.com/muety/wakapi/config"
	"github.com/muety/wakapi/models"
	"gorm.io/gorm"
	"log/slog"
)

func init() {
	f := migrationFunc{
		name: "20210206_drop_badges_column_add_sharing_flags",
		f: func(db *gorm.DB, cfg *config.Config) error {
			migrator := db.Migrator()

			if !migrator.HasColumn(&models.User{}, "badges_enabled") {
				// empty database or already migrated, nothing to migrate
				return nil
			}

			if err := db.Exec("UPDATE users SET share_data_max_days = 30 WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}
			if err := db.Exec("UPDATE users SET share_editors = TRUE WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}
			if err := db.Exec("UPDATE users SET share_languages = TRUE WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}
			if err := db.Exec("UPDATE users SET share_projects = TRUE WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}
			if err := db.Exec("UPDATE users SET share_oss = TRUE WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}
			if err := db.Exec("UPDATE users SET share_machines = TRUE WHERE badges_enabled = TRUE").Error; err != nil {
				return err
			}

			if cfg.Db.Dialect == config.SQLDialectSqlite {
				slog.Info("not attempting to drop column on sqlite", "column", "badges_enabled")
				return nil
			}

			if err := migrator.DropColumn(&models.User{}, "badges_enabled"); err != nil {
				return err
			}
			slog.Info("dropped column after substituting it by sharing indicators", "column", "badges_enabled")

			return nil
		},
	}

	registerPostMigration(f)
}
