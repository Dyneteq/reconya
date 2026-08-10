package alert

import (
	"context"
	"log"
	"sync"
	"time"

	"reconya/db"
	"reconya/models"
)

// oneShotMaxAge is how long an event-shaped alert (a new device, a newly opened
// port) stays in the feed before it auto-resolves. Those rules describe
// something that happened rather than something that is true, so they cannot be
// recomputed and resolved the way the stateful rules are.
//
// 24h matches the console's "NEW · 24H" metric.
const oneShotMaxAge = 24 * time.Hour

// offlineWindow bounds how long after a host goes quiet the host_unreachable
// rule keeps reporting it. Past this it is not a regression any more, just a
// device that is no longer on the network, and the host list already says so.
const offlineWindow = 7 * 24 * time.Hour

// DeviceReader is the slice of DeviceService the engine needs. Declaring it here
// rather than importing internal/device keeps the dependency one-directional —
// the same trick internal/portscan uses.
type DeviceReader interface {
	FindAll() ([]*models.Device, error)
}

// NetworkReader is the slice of NetworkService the engine needs.
type NetworkReader interface {
	FindAll() ([]models.Network, error)
}

// AlertService evaluates alert rules and reconciles their findings with what is
// already stored.
type AlertService struct {
	repository       db.AlertRepository
	deviceService    DeviceReader
	networkService   NetworkReader
	dbManager        *db.DBManager
	offlineThreshold time.Duration

	// evalMu serialises full evaluations. They are triggered from the scan loop
	// and from a periodic ticker, and two overlapping passes would race on the
	// auto-resolve step: the slower one could resolve findings the faster one
	// had just written.
	evalMu sync.Mutex
}

// NewAlertService creates a new alert service. offlineThreshold is how long a
// device must be unreachable before host_unreachable fires.
func NewAlertService(
	repo db.AlertRepository,
	deviceService DeviceReader,
	networkService NetworkReader,
	dbManager *db.DBManager,
	offlineThreshold time.Duration,
) *AlertService {
	if offlineThreshold <= 0 {
		offlineThreshold = 6 * time.Hour
	}

	return &AlertService{
		repository:       repo,
		deviceService:    deviceService,
		networkService:   networkService,
		dbManager:        dbManager,
		offlineThreshold: offlineThreshold,
	}
}

// Evaluate recomputes every stateful rule across all devices, upserts what it
// finds, and resolves alerts whose condition no longer holds.
//
// It deliberately looks at every device rather than just the network that was
// swept: auto-resolve is scoped per rule, so evaluating a subset would resolve
// perfectly valid findings on the networks that were not swept.
func (s *AlertService) Evaluate(ctx context.Context) error {
	s.evalMu.Lock()
	defer s.evalMu.Unlock()

	devices, err := s.deviceService.FindAll()
	if err != nil {
		return err
	}

	cidrs := map[string]string{}
	if networks, err := s.networkService.FindAll(); err == nil {
		for _, n := range networks {
			cidrs[n.ID] = n.CIDR
		}
	}

	findings := EvaluateStateful(EvalInput{
		Devices:          devices,
		NetworkCIDR:      cidrs,
		Now:              time.Now(),
		OfflineThreshold: s.offlineThreshold,
		OfflineWindow:    offlineWindow,
	})

	liveKeys := map[string][]string{}
	for _, rule := range StatefulRules {
		liveKeys[rule] = []string{}
	}

	for _, f := range findings {
		if err := s.write(ctx, f); err != nil {
			log.Printf("Alert: failed to write %s: %v", f.DedupeKey, err)
			continue
		}
		liveKeys[f.RuleID] = append(liveKeys[f.RuleID], f.DedupeKey)
	}

	for _, rule := range StatefulRules {
		if err := s.dbManager.ResolveMissingAlerts(s.repository, ctx, rule, liveKeys[rule]); err != nil {
			log.Printf("Alert: failed to resolve stale %s alerts: %v", rule, err)
		}
	}

	s.expireOneShots(ctx)

	return nil
}

// RecordNewDevice raises the new-device alert for a host seen for the first time.
func (s *AlertService) RecordNewDevice(ctx context.Context, device *models.Device, networkCIDR string) {
	if device == nil || device.ID == "" {
		return
	}

	if err := s.write(ctx, NewDeviceFinding(device, networkCIDR)); err != nil {
		log.Printf("Alert: failed to record new device %s: %v", device.IPv4, err)
	}
}

// RecordNewPorts raises an alert per port that was not open on the previous scan
// of this device.
func (s *AlertService) RecordNewPorts(ctx context.Context, device *models.Device, ports []models.Port) {
	if device == nil || device.ID == "" {
		return
	}

	for _, p := range ports {
		if err := s.write(ctx, NewPortFinding(device, p)); err != nil {
			log.Printf("Alert: failed to record new port %s on %s: %v", p.Number, device.IPv4, err)
		}
	}
}

// List returns alerts matching the query.
func (s *AlertService) List(ctx context.Context, q models.AlertQuery) ([]*models.Alert, error) {
	return s.repository.FindAll(ctx, q)
}

// Counts returns the per-severity breakdown of alerts that still want attention
// (open only — acked alerts stay visible in the feed but must not keep the
// header tile lit).
func (s *AlertService) Counts(ctx context.Context) (models.AlertCounts, error) {
	return s.repository.CountsBySeverity(ctx, []models.AlertState{models.AlertStateOpen})
}

// Ack acknowledges a single alert.
func (s *AlertService) Ack(ctx context.Context, id string) error {
	return s.repository.SetState(ctx, id, models.AlertStateAcked)
}

// AckAll acknowledges every open alert and reports how many it moved.
func (s *AlertService) AckAll(ctx context.Context) (int, error) {
	return s.repository.AckAll(ctx)
}

// write persists one finding, going through the DB manager so writes from scan
// goroutines stay serialised with everything else.
func (s *AlertService) write(ctx context.Context, f Finding) error {
	return s.dbManager.UpsertAlert(s.repository, ctx, &models.Alert{
		RuleID:    f.RuleID,
		DedupeKey: f.DedupeKey,
		Severity:  f.Severity,
		Title:     f.Title,
		Detail:    f.Detail,
		DeviceID:  f.DeviceID,
		NetworkID: f.NetworkID,
		State:     models.AlertStateOpen,
	})
}

// expireOneShots resolves event-shaped alerts once they are older than
// oneShotMaxAge. They have no recomputable condition, so age is the only thing
// that can retire them.
func (s *AlertService) expireOneShots(ctx context.Context) {
	cutoff := time.Now().Add(-oneShotMaxAge)
	oneShot := map[string]bool{RuleNewDevice: true, RuleNewPort: true}

	alerts, err := s.repository.FindAll(ctx, models.AlertQuery{
		States: []models.AlertState{models.AlertStateOpen, models.AlertStateAcked},
	})
	if err != nil {
		log.Printf("Alert: failed to list alerts for expiry: %v", err)
		return
	}

	for _, a := range alerts {
		if !oneShot[a.RuleID] || a.LastSeenAt.After(cutoff) {
			continue
		}
		if err := s.repository.SetState(ctx, a.ID, models.AlertStateResolved); err != nil {
			log.Printf("Alert: failed to expire %s: %v", a.DedupeKey, err)
		}
	}
}
