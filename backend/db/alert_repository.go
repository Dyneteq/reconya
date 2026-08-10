package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"reconya/models"
)

// alertColumns is the canonical SELECT list, kept in one place so every scan
// helper below stays in sync with scanAlert.
const alertColumns = `id, rule_id, dedupe_key, severity, title, detail, device_id, network_id,
	state, occurrences, first_seen_at, last_seen_at, acked_at, resolved_at, created_at, updated_at`

// SQLiteAlertRepository implements the AlertRepository interface for SQLite
type SQLiteAlertRepository struct {
	db *sql.DB
}

// NewSQLiteAlertRepository creates a new SQLiteAlertRepository
func NewSQLiteAlertRepository(db *sql.DB) *SQLiteAlertRepository {
	return &SQLiteAlertRepository{db: db}
}

// Close closes the database connection
func (r *SQLiteAlertRepository) Close() error {
	return r.db.Close()
}

// Upsert inserts an alert or, if one already exists for the same dedupe key,
// refreshes it.
//
// State is deliberately left alone on conflict: rules re-emit their whole
// finding set after every sweep, so bumping an acked alert back to open would
// make acknowledgement useless. A resolved alert whose condition returns is the
// one exception — that reopens.
//
// Occurrences counts recurrences, not sightings. Rules re-emit every finding on
// every pass, so counting sightings would just report how many times the
// evaluator has run — a host that has been unreachable overnight would read
// "×700". Incrementing only on the resolved→open transition makes the number
// mean "this has come back N times", which is worth showing.
func (r *SQLiteAlertRepository) Upsert(ctx context.Context, alert *models.Alert) error {
	now := time.Now()
	if alert.ID == "" {
		alert.ID = GenerateID()
	}
	if alert.FirstSeenAt.IsZero() {
		alert.FirstSeenAt = now
	}
	if alert.LastSeenAt.IsZero() {
		alert.LastSeenAt = now
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = now
	}
	alert.UpdatedAt = now
	if alert.State == "" {
		alert.State = models.AlertStateOpen
	}
	if alert.Occurrences <= 0 {
		alert.Occurrences = 1
	}

	query := `
	INSERT INTO alerts (` + alertColumns + `)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(dedupe_key) DO UPDATE SET
		severity    = excluded.severity,
		title       = excluded.title,
		detail      = excluded.detail,
		device_id   = excluded.device_id,
		network_id  = excluded.network_id,
		occurrences = alerts.occurrences + CASE WHEN alerts.state = 'resolved' THEN 1 ELSE 0 END,
		last_seen_at = excluded.last_seen_at,
		updated_at  = excluded.updated_at,
		state       = CASE WHEN alerts.state = 'resolved' THEN 'open' ELSE alerts.state END,
		resolved_at = CASE WHEN alerts.state = 'resolved' THEN NULL ELSE alerts.resolved_at END`

	_, err := r.db.ExecContext(ctx, query,
		alert.ID, alert.RuleID, alert.DedupeKey, string(alert.Severity), alert.Title, alert.Detail,
		nullableString(alert.DeviceID), nullableString(alert.NetworkID),
		string(alert.State), alert.Occurrences,
		alert.FirstSeenAt, alert.LastSeenAt,
		nullableTime(alert.AckedAt), nullableTime(alert.ResolvedAt),
		alert.CreatedAt, alert.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error upserting alert %s: %w", alert.DedupeKey, err)
	}

	return nil
}

// FindByID returns a single alert, or ErrNotFound.
func (r *SQLiteAlertRepository) FindByID(ctx context.Context, id string) (*models.Alert, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+alertColumns+` FROM alerts WHERE id = ?`, id)

	alert, err := scanAlert(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error finding alert by id: %w", err)
	}

	return alert, nil
}

// FindAll returns alerts matching the query, most severe first then most recent.
func (r *SQLiteAlertRepository) FindAll(ctx context.Context, q models.AlertQuery) ([]*models.Alert, error) {
	var where []string
	var args []interface{}

	if len(q.States) > 0 {
		where = append(where, "state IN ("+placeholders(len(q.States))+")")
		for _, s := range q.States {
			args = append(args, string(s))
		}
	}
	if len(q.Severities) > 0 {
		where = append(where, "severity IN ("+placeholders(len(q.Severities))+")")
		for _, s := range q.Severities {
			args = append(args, string(s))
		}
	}
	if q.DeviceID != "" {
		where = append(where, "device_id = ?")
		args = append(args, q.DeviceID)
	}

	query := `SELECT ` + alertColumns + ` FROM alerts`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// Acked sinks below open; within a state, most severe first, then newest.
	query += `
	ORDER BY
		CASE state WHEN 'open' THEN 0 WHEN 'acked' THEN 1 ELSE 2 END,
		CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 WHEN 'notice' THEN 2 ELSE 3 END,
		last_seen_at DESC`
	if q.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, q.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying alerts: %w", err)
	}
	defer rows.Close()

	alerts := []*models.Alert{}
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			return nil, fmt.Errorf("error scanning alert: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alerts: %w", err)
	}

	return alerts, nil
}

// SetState moves one alert through its lifecycle, stamping the matching
// timestamp column.
func (r *SQLiteAlertRepository) SetState(ctx context.Context, id string, state models.AlertState) error {
	now := time.Now()

	var query string
	switch state {
	case models.AlertStateAcked:
		query = `UPDATE alerts SET state = ?, acked_at = ?, updated_at = ? WHERE id = ?`
	case models.AlertStateResolved:
		query = `UPDATE alerts SET state = ?, resolved_at = ?, updated_at = ? WHERE id = ?`
	default:
		// Reopening clears both stamps.
		_, err := r.db.ExecContext(ctx,
			`UPDATE alerts SET state = ?, acked_at = NULL, resolved_at = NULL, updated_at = ? WHERE id = ?`,
			string(state), now, id)
		if err != nil {
			return fmt.Errorf("error setting alert state: %w", err)
		}
		return nil
	}

	res, err := r.db.ExecContext(ctx, query, string(state), now, now, id)
	if err != nil {
		return fmt.Errorf("error setting alert state: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}

	return nil
}

// AckAll acknowledges every currently open alert and reports how many it moved.
func (r *SQLiteAlertRepository) AckAll(ctx context.Context) (int, error) {
	now := time.Now()

	res, err := r.db.ExecContext(ctx,
		`UPDATE alerts SET state = 'acked', acked_at = ?, updated_at = ? WHERE state = 'open'`,
		now, now)
	if err != nil {
		return 0, fmt.Errorf("error acknowledging alerts: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return int(n), nil
}

// ResolveMissing resolves alerts of a rule whose condition no longer holds —
// i.e. every non-resolved alert for the rule whose dedupe key was absent from
// the evaluation that just ran.
//
// liveKeys being empty is meaningful (the rule found nothing at all), so it
// resolves everything for that rule rather than short-circuiting.
func (r *SQLiteAlertRepository) ResolveMissing(ctx context.Context, ruleID string, liveKeys []string) (int, error) {
	now := time.Now()

	query := `UPDATE alerts SET state = 'resolved', resolved_at = ?, updated_at = ?
			  WHERE rule_id = ? AND state != 'resolved'`
	args := []interface{}{now, now, ruleID}

	if len(liveKeys) > 0 {
		query += " AND dedupe_key NOT IN (" + placeholders(len(liveKeys)) + ")"
		for _, k := range liveKeys {
			args = append(args, k)
		}
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("error resolving stale alerts for rule %s: %w", ruleID, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return int(n), nil
}

// CountsBySeverity breaks down alerts in the given states by severity. This
// feeds the console header tile, which counts open + acked separately.
func (r *SQLiteAlertRepository) CountsBySeverity(ctx context.Context, states []models.AlertState) (models.AlertCounts, error) {
	var counts models.AlertCounts

	query := `SELECT severity, COUNT(*) FROM alerts`
	var args []interface{}
	if len(states) > 0 {
		query += " WHERE state IN (" + placeholders(len(states)) + ")"
		for _, s := range states {
			args = append(args, string(s))
		}
	}
	query += " GROUP BY severity"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return counts, fmt.Errorf("error counting alerts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var severity string
		var n int
		if err := rows.Scan(&severity, &n); err != nil {
			return counts, fmt.Errorf("error scanning alert counts: %w", err)
		}
		switch models.AlertSeverity(severity) {
		case models.AlertCritical:
			counts.Critical = n
		case models.AlertWarning:
			counts.Warning = n
		case models.AlertNotice:
			counts.Notice = n
		case models.AlertInfo:
			counts.Info = n
		}
	}

	return counts, rows.Err()
}

// DeleteByDeviceID removes every alert attached to a device. The foreign key
// cascades on device deletion, but callers that delete devices through a path
// where foreign keys may be off can use this directly.
func (r *SQLiteAlertRepository) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM alerts WHERE device_id = ?`, deviceID)
	if err != nil {
		return fmt.Errorf("error deleting alerts for device %s: %w", deviceID, err)
	}

	return nil
}

// rowScanner covers both *sql.Row and *sql.Rows so scanAlert serves FindByID
// and FindAll alike.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAlert(row rowScanner) (*models.Alert, error) {
	var alert models.Alert
	var severity, state string
	var deviceID, networkID sql.NullString
	var ackedAt, resolvedAt sql.NullTime

	err := row.Scan(
		&alert.ID, &alert.RuleID, &alert.DedupeKey, &severity, &alert.Title, &alert.Detail,
		&deviceID, &networkID, &state, &alert.Occurrences,
		&alert.FirstSeenAt, &alert.LastSeenAt, &ackedAt, &resolvedAt,
		&alert.CreatedAt, &alert.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	alert.Severity = models.AlertSeverity(severity)
	alert.State = models.AlertState(state)
	if deviceID.Valid {
		alert.DeviceID = &deviceID.String
	}
	if networkID.Valid {
		alert.NetworkID = &networkID.String
	}
	if ackedAt.Valid {
		alert.AckedAt = &ackedAt.Time
	}
	if resolvedAt.Valid {
		alert.ResolvedAt = &resolvedAt.Time
	}

	return &alert, nil
}

// placeholders builds "?, ?, ?" for an IN clause of n values.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
