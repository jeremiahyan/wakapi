package services

import (
	"github.com/duke-git/lancet/v2/datetime"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/muety/wakapi/config"
	"github.com/muety/wakapi/models"
	"time"
)

type DurationService struct {
	config           *config.Config
	heartbeatService IHeartbeatService
}

func NewDurationService(heartbeatService IHeartbeatService) *DurationService {
	srv := &DurationService{
		config:           config.Get(),
		heartbeatService: heartbeatService,
	}
	return srv
}

func (srv *DurationService) Get(from, to time.Time, user *models.User, filters *models.Filters) (models.Durations, error) {
	heartbeatsTimeout := user.HeartbeatsTimeout()

	heartbeats, err := srv.heartbeatService.GetAllWithin(from, to, user)
	if err != nil {
		return nil, err
	}

	// Aggregation
	// the below logic is approximately equivalent to the SQL query at scripts/aggregate_durations_mysql.sql
	// a postgres-compatible script was contributed by @cwilby and is available at scripts/aggregate_durations_postgres.sql
	// i'm hesitant to replicate that logic for sqlite and mssql too (because probably painful to impossible), but we could
	// think about adding a distrinctio here to use pure-sql aggregation for mysql and postgres, and traditional, programmatic
	// aggregation for all other databases
	var count int
	var latest *models.Duration

	mapping := make(map[string][]*models.Duration)

	for _, h := range heartbeats {
		d1 := models.NewDurationFromHeartbeat(h).WithEntityIgnored().Hashed()

		if list, ok := mapping[d1.GroupHash]; !ok || len(list) < 1 {
			mapping[d1.GroupHash] = []*models.Duration{d1}
		}

		if latest == nil {
			latest = d1
			continue
		}

		sameDay := datetime.BeginOfDay(d1.Time.T()) == datetime.BeginOfDay(latest.Time.T())
		dur := time.Duration(mathutil.Min(
			int64(d1.Time.T().Sub(latest.Time.T().Add(latest.Duration))),
			int64(heartbeatsTimeout),
		))

		// skip heartbeats that span across two adjacent summaries (assuming there are no more than 1 summary per day)
		// this is relevant to prevent the time difference between generating summaries from raw heartbeats and aggregating pre-generated summaries
		// for the latter case, the very last heartbeat of a day won't be counted, so we don't want to count it here either
		// another option would be to adapt the Summarize() method to always append up to DefaultHeartbeatsTimeout seconds to a day's very last duration
		if !sameDay {
			dur = 0
		}
		latest.Duration += dur

		// start new "group" if:
		// (a) heartbeats were too far apart each other,
		// (b) if they are of a different entity or,
		// (c) if they span across two days
		if dur >= heartbeatsTimeout || latest.GroupHash != d1.GroupHash || !sameDay {
			list := mapping[d1.GroupHash]
			if d0 := list[len(list)-1]; d0 != d1 {
				mapping[d1.GroupHash] = append(mapping[d1.GroupHash], d1)
			}
			latest = d1
		} else {
			latest.NumHeartbeats++
		}

		count++
	}

	durations := make(models.Durations, 0)

	for _, list := range mapping {
		for _, d := range list {
			// even when filters are applied, we'll still have to compute the whole summary first and then filter out non-matching durations
			// if we fetched only matching heartbeats in the first place, there will be false positive gaps (see DefaultHeartbeatsTimeout)
			// in case the user worked on different projects in parallel
			// see https://github.com/muety/wakapi/issues/535
			if filters != nil && !filters.MatchDuration(d) {
				continue
			}

			if user.ExcludeUnknownProjects && d.Project == "" {
				continue
			}

			// will only happen if two heartbeats with different hashes (e.g. different project) have the same timestamp
			// that, in turn, will most likely only happen for mysql, where `time` column's precision was set to second for a while
			// assume that two non-identical heartbeats with identical time are sub-second apart from each other, so round up to expectancy value
			// also see https://github.com/muety/wakapi/issues/340
			if d.Duration == 0 {
				d.Duration = 500 * time.Millisecond
			}
			durations = append(durations, d)
		}
	}

	if len(heartbeats) == 1 && len(durations) == 1 {
		durations[0].Duration = heartbeatsTimeout
	}

	return durations.Sorted(), nil
}
