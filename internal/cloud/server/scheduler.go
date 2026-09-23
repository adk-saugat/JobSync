package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/saugatadhikari/jobSync/internal/cloud/service"
	"github.com/saugatadhikari/jobSync/internal/cloud/store"
)

const (
	defaultSyncCron = "30 21 * * *"
	defaultSyncTZ   = "America/Chicago"
)

type cronConfig struct {
	Spec     string
	Timezone string
	Disabled bool
}

func loadCronConfig() cronConfig {
	spec := strings.TrimSpace(os.Getenv("SYNC_CRON"))
	if spec == "" {
		spec = defaultSyncCron
	}
	tz := strings.TrimSpace(os.Getenv("SYNC_CRON_TZ"))
	if tz == "" {
		tz = defaultSyncTZ
	}
	return cronConfig{
		Spec:     spec,
		Timezone: tz,
		Disabled: envTruthy("SYNC_CRON_DISABLE"),
	}
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// StartScheduler runs the nightly sync-all job in-process with gocron.
// Cloud Run must keep CPU allocated (min instances + no CPU throttling)
// or the job will not fire when the service is idle.
func (s *Server) StartScheduler() (stop func() error, err error) {
	cfg := loadCronConfig()
	if cfg.Disabled {
		log.Printf("gocron disabled (SYNC_CRON_DISABLE)")
		return func() error { return nil }, nil
	}
	if s.DB == nil {
		return nil, fmt.Errorf("scheduler requires a database")
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("SYNC_CRON_TZ %q: %w", cfg.Timezone, err)
	}

	sched, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, fmt.Errorf("gocron: %w", err)
	}

	job, err := sched.NewJob(
		gocron.CronJob(cfg.Spec, false),
		gocron.NewTask(s.runCronSyncAll),
		gocron.WithName("sync-all"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		_ = sched.Shutdown()
		return nil, fmt.Errorf("gocron job: %w", err)
	}

	s.syncJob = job
	sched.Start()
	if next, nextErr := job.NextRun(); nextErr == nil {
		log.Printf("gocron started schedule=%q tz=%s next=%s", cfg.Spec, cfg.Timezone, next.Format(time.RFC3339))
	} else {
		log.Printf("gocron started schedule=%q tz=%s", cfg.Spec, cfg.Timezone)
	}
	return sched.Shutdown, nil
}

// nextSyncRun is RFC3339, or empty when the scheduler is off.
func (s *Server) nextSyncRun() string {
	if s.syncJob == nil {
		return ""
	}
	next, err := s.syncJob.NextRun()
	if err != nil || next.IsZero() {
		return ""
	}
	return next.Format(time.RFC3339)
}

func (s *Server) runCronSyncAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()

	unlock, ok, err := s.DB.TryAdvisoryLock(ctx, store.DailySyncLockKey)
	if err != nil {
		log.Printf("gocron lock error: %v", err)
		return
	}
	if !ok {
		log.Printf("gocron skipped: another instance holds the lock")
		return
	}
	defer unlock()

	log.Printf("gocron sync/all start")
	summary, err := service.RunCloudSyncAll(ctx, s.DB, service.DefaultSyncLimit, false, func(format string, args ...any) {
		log.Printf("sync: "+format, args...)
	})
	if err != nil {
		log.Printf("gocron sync/all error: %v", err)
		return
	}
	log.Printf("gocron sync/all done accounts=%d", summary.Accounts)
}
