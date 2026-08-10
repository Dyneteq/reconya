package integration

import (
	"context"
	"testing"
	"time"

	"reconya/db"
	"reconya/models"
	"reconya/tests/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAlert(ruleID, dedupeKey string, severity models.AlertSeverity) *models.Alert {
	return &models.Alert{
		RuleID:    ruleID,
		DedupeKey: dedupeKey,
		Severity:  severity,
		Title:     "Test alert " + dedupeKey,
		Detail:    "detail for " + dedupeKey,
	}
}

func TestAlertRepository_Integration(t *testing.T) {
	factory, cleanup := testutils.SetupTestRepositoryFactory(t)
	defer cleanup()

	alertRepo := factory.NewAlertRepository()
	ctx := context.Background()

	t.Run("UpsertIsIdempotentOnDedupeKey", func(t *testing.T) {
		a := newAlert("risky_port", "risky_port:d1:23", models.AlertWarning)
		require.NoError(t, alertRepo.Upsert(ctx, a))

		// Re-emitting the same finding must update, not duplicate. Rules re-emit
		// on every pass, so a re-sighting is the normal case and must not inflate
		// the occurrence counter.
		again := newAlert("risky_port", "risky_port:d1:23", models.AlertWarning)
		require.NoError(t, alertRepo.Upsert(ctx, again))

		found, err := alertRepo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)

		matching := 0
		for _, f := range found {
			if f.DedupeKey == "risky_port:d1:23" {
				matching++
				assert.Equal(t, 1, f.Occurrences, "re-sighting is not a recurrence")
			}
		}
		assert.Equal(t, 1, matching, "dedupe key must yield exactly one row")
	})

	t.Run("OccurrencesCountRecurrences", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()
		repo := factory.NewAlertRepository()

		// Two full away-and-back cycles.
		for i := 0; i < 3; i++ {
			require.NoError(t, repo.Upsert(ctx, newAlert("flap", "flap:d9", models.AlertWarning)))
			require.NoError(t, repo.Upsert(ctx, newAlert("flap", "flap:d9", models.AlertWarning)))
			if i < 2 {
				_, err := repo.ResolveMissing(ctx, "flap", nil)
				require.NoError(t, err)
			}
		}

		all, err := repo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)
		require.Len(t, all, 1)
		assert.Equal(t, 3, all[0].Occurrences, "one per appearance, not per evaluation")
	})

	t.Run("AckSurvivesReEmission", func(t *testing.T) {
		a := newAlert("host_unreachable", "host_unreachable:d2", models.AlertWarning)
		require.NoError(t, alertRepo.Upsert(ctx, a))

		stored, err := alertRepo.FindAll(ctx, models.AlertQuery{DeviceID: ""})
		require.NoError(t, err)

		var id string
		for _, s := range stored {
			if s.DedupeKey == "host_unreachable:d2" {
				id = s.ID
			}
		}
		require.NotEmpty(t, id)

		require.NoError(t, alertRepo.SetState(ctx, id, models.AlertStateAcked))

		// The rule fires again on the next evaluation; that must not undo the ack.
		require.NoError(t, alertRepo.Upsert(ctx, newAlert("host_unreachable", "host_unreachable:d2", models.AlertWarning)))

		after, err := alertRepo.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, models.AlertStateAcked, after.State)
		assert.NotNil(t, after.AckedAt)
	})

	t.Run("ResolveMissingClearsStaleFindingsOnly", func(t *testing.T) {
		require.NoError(t, alertRepo.Upsert(ctx, newAlert("dup_test", "dup_test:a", models.AlertCritical)))
		require.NoError(t, alertRepo.Upsert(ctx, newAlert("dup_test", "dup_test:b", models.AlertCritical)))

		// Next evaluation only re-emits "a".
		n, err := alertRepo.ResolveMissing(ctx, "dup_test", []string{"dup_test:a"})
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		open, err := alertRepo.FindAll(ctx, models.AlertQuery{
			States: []models.AlertState{models.AlertStateOpen},
		})
		require.NoError(t, err)

		keys := map[string]bool{}
		for _, o := range open {
			keys[o.DedupeKey] = true
		}
		assert.True(t, keys["dup_test:a"], "re-emitted finding stays open")
		assert.False(t, keys["dup_test:b"], "absent finding resolves")
	})

	t.Run("ResolvedAlertReopensWhenConditionReturns", func(t *testing.T) {
		require.NoError(t, alertRepo.Upsert(ctx, newAlert("flap", "flap:d3", models.AlertWarning)))
		_, err := alertRepo.ResolveMissing(ctx, "flap", nil)
		require.NoError(t, err)

		require.NoError(t, alertRepo.Upsert(ctx, newAlert("flap", "flap:d3", models.AlertWarning)))

		found, err := alertRepo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)

		var reopened *models.Alert
		for _, f := range found {
			if f.DedupeKey == "flap:d3" {
				reopened = f
			}
		}
		require.NotNil(t, reopened)
		assert.Equal(t, models.AlertStateOpen, reopened.State)
		assert.Nil(t, reopened.ResolvedAt, "resolved_at must be cleared on reopen")
	})

	t.Run("CountsBySeverityCoversOpenOnly", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()
		repo := factory.NewAlertRepository()

		require.NoError(t, repo.Upsert(ctx, newAlert("r", "k1", models.AlertCritical)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "k2", models.AlertCritical)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "k3", models.AlertWarning)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "k4", models.AlertNotice)))

		all, err := repo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)
		for _, a := range all {
			if a.DedupeKey == "k2" {
				require.NoError(t, repo.SetState(ctx, a.ID, models.AlertStateAcked))
			}
		}

		counts, err := repo.CountsBySeverity(ctx, []models.AlertState{models.AlertStateOpen})
		require.NoError(t, err)
		assert.Equal(t, 1, counts.Critical, "acked alert should drop out of the header count")
		assert.Equal(t, 1, counts.Warning)
		assert.Equal(t, 1, counts.Notice)
		assert.Equal(t, 3, counts.Total())
	})

	t.Run("AckAllMovesEveryOpenAlert", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()
		repo := factory.NewAlertRepository()

		require.NoError(t, repo.Upsert(ctx, newAlert("r", "x1", models.AlertCritical)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "x2", models.AlertWarning)))

		n, err := repo.AckAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		counts, err := repo.CountsBySeverity(ctx, []models.AlertState{models.AlertStateOpen})
		require.NoError(t, err)
		assert.Equal(t, 0, counts.Total())
	})

	t.Run("FindByIDReportsMissingRows", func(t *testing.T) {
		_, err := alertRepo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, db.ErrNotFound)
	})

	t.Run("OrdersOpenBeforeAckedAndBySeverity", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()
		repo := factory.NewAlertRepository()

		require.NoError(t, repo.Upsert(ctx, newAlert("r", "o-notice", models.AlertNotice)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "o-critical", models.AlertCritical)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "o-warning", models.AlertWarning)))
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "acked-critical", models.AlertCritical)))

		all, err := repo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)
		for _, a := range all {
			if a.DedupeKey == "acked-critical" {
				require.NoError(t, repo.SetState(ctx, a.ID, models.AlertStateAcked))
			}
		}

		ordered, err := repo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)

		keys := make([]string, 0, len(ordered))
		for _, a := range ordered {
			keys = append(keys, a.DedupeKey)
		}
		assert.Equal(t, []string{"o-critical", "o-warning", "o-notice", "acked-critical"}, keys)
	})

	t.Run("DeletingADeviceCascadesItsAlerts", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()

		deviceRepo := factory.NewDeviceRepository()
		repo := factory.NewAlertRepository()

		saved, err := deviceRepo.CreateOrUpdate(ctx, createTestDevice("192.168.1.240", "Doomed"))
		require.NoError(t, err)
		require.NotEmpty(t, saved.ID)

		a := newAlert("risky_port", "risky_port:"+saved.ID+":23", models.AlertWarning)
		a.DeviceID = &saved.ID
		require.NoError(t, repo.Upsert(ctx, a))

		require.NoError(t, deviceRepo.DeleteByID(ctx, saved.ID))

		remaining, err := repo.FindAll(ctx, models.AlertQuery{DeviceID: saved.ID})
		require.NoError(t, err)
		assert.Empty(t, remaining, "alerts should not outlive the device they point at")
	})

	t.Run("TimestampsRoundTrip", func(t *testing.T) {
		factory, cleanup := testutils.SetupTestRepositoryFactory(t)
		defer cleanup()
		repo := factory.NewAlertRepository()

		before := time.Now().Add(-time.Second)
		require.NoError(t, repo.Upsert(ctx, newAlert("r", "ts", models.AlertInfo)))

		all, err := repo.FindAll(ctx, models.AlertQuery{})
		require.NoError(t, err)
		require.Len(t, all, 1)

		assert.False(t, all[0].FirstSeenAt.IsZero())
		assert.False(t, all[0].LastSeenAt.IsZero())
		assert.True(t, all[0].CreatedAt.After(before))
	})
}
