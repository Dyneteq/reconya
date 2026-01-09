package sensor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"reconya/db"
	"reconya/models"
)

type SensorService struct {
	repo db.SensorRepository
}

func NewSensorService(repo db.SensorRepository) *SensorService {
	return &SensorService{repo: repo}
}

func (s *SensorService) CreateSensor(ctx context.Context, name string) (*models.Sensor, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	sensor := &models.Sensor{
		Name:       name,
		Token:      token,
		Status:     models.SensorStatusPending,
		ScanStatus: models.SensorScanStatusIdle,
	}

	return s.repo.Create(ctx, sensor)
}

func (s *SensorService) GetAllSensors(ctx context.Context) ([]*models.Sensor, error) {
	return s.repo.FindAll(ctx)
}

func (s *SensorService) GetSensorByID(ctx context.Context, id string) (*models.Sensor, error) {
	return s.repo.FindByID(ctx, id)
}

// GetSensorByToken returns a sensor matching the provided token.
func (s *SensorService) GetSensorByToken(ctx context.Context, token string) (*models.Sensor, error) {
	return s.repo.FindByToken(ctx, token)
}

func (s *SensorService) DeleteSensor(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type RegisterRequest struct {
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
	PublicIP    string `json:"public_ip"`
	MAC         string `json:"mac"`
	Interface   string `json:"interface"`
	NetworkCIDR string `json:"network_cidr"`
}

func (s *SensorService) Register(ctx context.Context, token string, req RegisterRequest) (*models.Sensor, error) {
	sensor, err := s.repo.FindByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	now := time.Now()
	sensor.Status = models.SensorStatusOnline
	sensor.LastSeenAt = &now

	if sensor.RegisteredAt == nil {
		sensor.RegisteredAt = &now
	}

	if req.Hostname != "" {
		sensor.Hostname = &req.Hostname
	}
	if req.IP != "" {
		sensor.IP = &req.IP
	}
	if req.PublicIP != "" {
		sensor.PublicIP = &req.PublicIP
	}
	if req.MAC != "" {
		sensor.MAC = &req.MAC
	}
	if req.Interface != "" {
		sensor.Interface = &req.Interface
	}
	if req.NetworkCIDR != "" {
		sensor.NetworkCIDR = &req.NetworkCIDR
	}

	return s.repo.Update(ctx, sensor)
}

func (s *SensorService) UpdateSensorStatuses(ctx context.Context, offlineThreshold time.Duration) error {
	sensors, err := s.repo.FindAll(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, sensor := range sensors {
		if sensor.Status == models.SensorStatusOnline && sensor.LastSeenAt != nil {
			if now.Sub(*sensor.LastSeenAt) > offlineThreshold {
				sensor.Status = models.SensorStatusOffline
				_, err := s.repo.Update(ctx, sensor)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// UpdateScanStatus updates the desired scan status for a sensor and records timing metadata.
func (s *SensorService) UpdateScanStatus(ctx context.Context, sensorID string, status models.SensorScanStatus) (*models.Sensor, error) {
	sensor, err := s.repo.FindByID(ctx, sensorID)
	if err != nil {
		return nil, err
	}
	if sensor == nil {
		return nil, fmt.Errorf("sensor not found")
	}

	now := time.Now()
	sensor.ScanStatus = status

	switch status {
	case models.SensorScanStatusRunning:
		sensor.LastScanStartedAt = &now
		sensor.LastScanError = nil
	case models.SensorScanStatusStopped, models.SensorScanStatusIdle:
		sensor.LastScanCompletedAt = &now
	}

	return s.repo.Update(ctx, sensor)
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
