package repositories

import (
	"database/sql"
	"time"

	conf "github.com/muety/wakapi/config"
	"github.com/muety/wakapi/models"
	"github.com/muety/wakapi/utils"
	"gorm.io/gorm"
)

type HeartbeatRepository struct {
	BaseRepository
	config *conf.Config
}

func NewHeartbeatRepository(db *gorm.DB) *HeartbeatRepository {
	return &HeartbeatRepository{BaseRepository: NewBaseRepository(db), config: conf.Get()}
}

// Use with caution!!
func (r *HeartbeatRepository) GetAll() ([]*models.Heartbeat, error) {
	var heartbeats []*models.Heartbeat
	if err := r.db.Find(&heartbeats).Error; err != nil {
		return nil, err
	}
	return heartbeats, nil
}

func (r *HeartbeatRepository) InsertBatch(heartbeats []*models.Heartbeat) error {
	return InsertBatchChunked[*models.Heartbeat](heartbeats, &models.Heartbeat{}, r.db)
}

func (r *HeartbeatRepository) GetLatestByUser(user *models.User) (*models.Heartbeat, error) {
	var heartbeat models.Heartbeat
	if err := r.db.
		Model(&models.Heartbeat{}).
		Where(&models.Heartbeat{UserID: user.ID}).
		Order("time desc").
		Limit(1).
		Scan(&heartbeat).Error; err != nil {
		return nil, err
	}
	return &heartbeat, nil
}

func (r *HeartbeatRepository) GetLatestByOriginAndUser(origin string, user *models.User) (*models.Heartbeat, error) {
	var heartbeat models.Heartbeat
	if err := r.db.
		Model(&models.Heartbeat{}).
		Where(&models.Heartbeat{
			UserID: user.ID,
			Origin: origin,
		}).
		Order("time desc").
		Limit(1).
		Scan(&heartbeat).Error; err != nil {
		return nil, err
	}
	return &heartbeat, nil
}

func (r *HeartbeatRepository) GetAllWithin(from, to time.Time, user *models.User) ([]*models.Heartbeat, error) {
	// https://stackoverflow.com/a/20765152/3112139
	var heartbeats []*models.Heartbeat
	if err := r.db.
		Where(&models.Heartbeat{UserID: user.ID}).
		Where("time >= ?", from.Local()).
		Where("time < ?", to.Local()).
		Order("time asc").
		Find(&heartbeats).Error; err != nil {
		return nil, err
	}
	return heartbeats, nil
}

func (r *HeartbeatRepository) StreamAllWithin(from, to time.Time, user *models.User) (chan *models.Heartbeat, error) {
	out := make(chan *models.Heartbeat)

	rows, err := r.db.
		Model(&models.Heartbeat{}).
		Where(&models.Heartbeat{UserID: user.ID}).
		Where("time >= ?", from.Local()).
		Where("time < ?", to.Local()).
		Order("time asc").
		Rows()

	if err != nil {
		return nil, err
	}

	go streamRows[models.Heartbeat](rows, out, r.db, func(err error) {
		conf.Log().Error("failed to scan heartbeats row", "user", user.ID, "from", from, "to", to, "error", err)
	})

	return out, nil
}

func (r *HeartbeatRepository) GetAllWithinByFilters(from, to time.Time, user *models.User, filterMap map[string][]string) ([]*models.Heartbeat, error) {
	// https://stackoverflow.com/a/20765152/3112139
	var heartbeats []*models.Heartbeat

	q := r.db.
		Where(&models.Heartbeat{UserID: user.ID}).
		Where("time >= ?", from.Local()).
		Where("time < ?", to.Local()).
		Order("time asc")
	q = filteredQuery(q, filterMap)

	if err := q.Find(&heartbeats).Error; err != nil {
		return nil, err
	}
	return heartbeats, nil
}

func (r *HeartbeatRepository) StreamAllWithinByFilters(from, to time.Time, user *models.User, filterMap map[string][]string) (chan *models.Heartbeat, error) {
	out := make(chan *models.Heartbeat)

	q := r.db.
		Where(&models.Heartbeat{UserID: user.ID}).
		Where("time >= ?", from.Local()).
		Where("time < ?", to.Local()).
		Order("time asc")
	q = filteredQuery(q, filterMap)

	rows, err := q.Rows()
	if err != nil {
		return nil, err
	}

	go streamRows[models.Heartbeat](rows, out, r.db, func(err error) {
		conf.Log().Error("failed to scan filtered heartbeats row", "user", user.ID, "from", from, "to", to, "error", err)
	})

	return out, nil
}

func (r *HeartbeatRepository) GetLatestByFilters(user *models.User, filterMap map[string][]string) (*models.Heartbeat, error) {
	var heartbeat *models.Heartbeat

	q := r.db.
		Where(&models.Heartbeat{UserID: user.ID}).
		Order("time desc")
	q = filteredQuery(q, filterMap)

	if err := q.Limit(1).Scan(&heartbeat).Error; err != nil {
		return nil, err
	}
	return heartbeat, nil
}

func (r *HeartbeatRepository) GetFirstByUsers() ([]*models.TimeByUser, error) {
	var result []*models.TimeByUser
	r.db.Raw("with agg as (select " + utils.QuoteSql(r.db, "user_id, min(time) as %s", "time") + " from heartbeats group by user_id) " +
		"select " + utils.QuoteSql(r.db, "id as %s, time ", "user") +
		"from users " +
		"left join agg on agg.user_id = id " +
		"order by users.id").
		Scan(&result)
	return result, nil
}

func (r *HeartbeatRepository) GetLastByUsers() ([]*models.TimeByUser, error) {
	var result []*models.TimeByUser
	r.db.Raw("with agg as (select " + utils.QuoteSql(r.db, "user_id, max(time) as %s", "time") + " from heartbeats group by user_id) " +
		"select " + utils.QuoteSql(r.db, "id as %s, time ", "user") +
		"from users " +
		"left join agg on agg.user_id = id " +
		"order by users.id").
		Scan(&result)
	return result, nil
}

func (r *HeartbeatRepository) Count(approximate bool) (count int64, err error) {
	if r.config.Db.IsMySQL() && approximate {
		err = r.db.Table("information_schema.tables").
			Select("table_rows").
			Where("table_schema = ?", r.config.Db.Name).
			Where("table_name = 'heartbeats'").
			Scan(&count).Error
	}

	if count == 0 {
		err = r.db.
			Model(&models.Heartbeat{}).
			Count(&count).Error
	}
	return count, nil
}

func (r *HeartbeatRepository) CountByUser(user *models.User) (int64, error) {
	var count int64
	if err := r.db.
		Model(&models.Heartbeat{}).
		Where(&models.Heartbeat{UserID: user.ID}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *HeartbeatRepository) CountByUsers(users []*models.User) ([]*models.CountByUser, error) {
	var counts []*models.CountByUser

	userIds := make([]string, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	if len(userIds) == 0 {
		return counts, nil
	}

	if err := r.db.
		Model(&models.Heartbeat{}).
		Select(utils.QuoteSql(r.db, "user_id as %s, count(id) as %s", "user", "count")).
		Where("user_id in ?", userIds).
		Group("user").
		Find(&counts).Error; err != nil {
		return counts, err
	}

	return counts, nil
}

func (r *HeartbeatRepository) GetEntitySetByUser(entityType uint8, userId string) ([]string, error) {
	var results []string
	if err := r.db.
		Model(&models.Heartbeat{}).
		Distinct(models.GetEntityColumn(entityType)).
		Where(&models.Heartbeat{UserID: userId}).
		Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *HeartbeatRepository) DeleteBefore(t time.Time) error {
	if err := r.db.
		Where("time <= ?", t.Local()).
		Delete(models.Heartbeat{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *HeartbeatRepository) DeleteByUser(user *models.User) error {
	if err := r.db.
		Where("user_id = ?", user.ID).
		Delete(models.Heartbeat{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *HeartbeatRepository) DeleteByUserBefore(user *models.User, t time.Time) error {
	if err := r.db.
		Where("user_id = ?", user.ID).
		Where("time <= ?", t.Local()).
		Delete(models.Heartbeat{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *HeartbeatRepository) GetUserProjectStats(user *models.User, from, to time.Time, limit, offset int) ([]*models.ProjectStats, error) {
	var projectStats []*models.ProjectStats

	// note: limit / offset doesn't really improve query performance
	// query takes quite long, depending on the number of heartbeats (~ 7 seconds for ~ 500k heartbeats)
	// TODO: refactor this to use summaries once we implemented persisting filtered, multi-interval summaries
	// see https://github.com/muety/wakapi/issues/524#issuecomment-1731668391

	// multi-line string with backticks yields an error with the github.com/glebarez/sqlite driver

	args := []interface{}{
		sql.Named("userid", user.ID),
		sql.Named("from", from.Format(time.RFC3339)),
		sql.Named("to", to.Format(time.RFC3339)),
		sql.Named("limit", limit),
		sql.Named("offset", offset),
	}

	limitOffsetClause := "limit @limit offset @offset"
	if r.config.Db.IsMssql() {
		limitOffsetClause = "offset @offset ROWS fetch next @limit rows only"
	}

	query := `
			with project_stats as (
				select
					project,
					user_id,
					min(time) as first,
					max(time) as last,
					count(*) as cnt
				from heartbeats
				where user_id = @userid
				  and project != ''
				  and time between @from and @to
				  and language is not null and language != ''
				group by project, user_id
			),
				 language_stats as (
					 select
						 project,
						 language,
						 count(*) as language_count,
						 row_number() over (partition by project order by count(*) desc) as rn
					 from heartbeats
					 where user_id = @userid
					   and project != ''
					   and time between @from and @to
					   and language is not null and language != ''
					 group by project, language
				 )
			select
				ps.project,
				ps.first,
				ps.last,
				ps.cnt as count,
				ls.language as top_language
			from project_stats ps
					 left join language_stats ls on ps.project = ls.project and ls.rn = 1
			order by ps.last desc
	`

	query += limitOffsetClause

	if err := r.db.
		Raw(query, args...).
		Scan(&projectStats).Error; err != nil {
		return nil, err
	}

	return projectStats, nil
}
