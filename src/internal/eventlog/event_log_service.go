package eventlog

import (
	"context"
	"fmt"
	"log"
	"reconya/db"
	"reconya/internal/device"
	"reconya/models"
	"time"
)

type EventLogService struct {
	repository    db.EventLogRepository
	DeviceService *device.DeviceService
	dbManager     *db.DBManager
}

func NewEventLogService(repository db.EventLogRepository, deviceService *device.DeviceService, dbManager *db.DBManager) *EventLogService {
	return &EventLogService{
		repository:    repository,
		DeviceService: deviceService,
		dbManager:     dbManager,
	}
}

func (s *EventLogService) GetAll(limitSize int64) ([]models.EventLog, error) {
	ctx := context.Background()
	eventLogPtrs, err := s.repository.FindLatest(ctx, int(limitSize))
	if err != nil {
		return nil, err
	}

	eventLogs := make([]models.EventLog, len(eventLogPtrs))
	for i, logPtr := range eventLogPtrs {
		eventLogs[i] = *logPtr
		eventLogs[i].Description = s.generateDescription(eventLogs[i])
	}
	
	return eventLogs, nil
}

func (s *EventLogService) generateDescription(eventLog models.EventLog) string {
	deviceInfo := "unknown device"
	if eventLog.DeviceID != nil {
		device, err := s.DeviceService.FindByID(*eventLog.DeviceID)
		if err != nil {
			log.Printf("Error fetching device information: %v", err)
		} else if device != nil && device.IPv4 != "" {
			deviceInfo = device.IPv4
		}
	}

			switch eventLog.Type {
		case models.PingSweep:
			if eventLog.DurationSeconds != nil {
				return fmt.Sprintf("Ping sweep completed in %d seconds", int(*eventLog.DurationSeconds))
			} else {
				return "Ping sweep performed"
			}
	case models.PortScanStarted:
		return fmt.Sprintf("Port scan started for [%s]", deviceInfo)
	case models.PortScanCompleted:
		return fmt.Sprintf("Port scan completed [%s]", deviceInfo)
	case models.DeviceOnline:
		return fmt.Sprintf("Live device [%s] found", deviceInfo)
	case models.DeviceIdle:
		return fmt.Sprintf("Device [%s] became idle", deviceInfo)
	case models.DeviceOffline:
		return fmt.Sprintf("Device [%s] is now offline", deviceInfo)
	case models.LocalIPFound:
		return fmt.Sprintf("Local IPv4 address found [%s]", deviceInfo)
	case models.LocalNetworkFound:
		return "Local network found"
	case models.NetworkCreated:
		return eventLog.Description // Use the custom description for network events
	case models.NetworkUpdated:
		return eventLog.Description // Use the custom description for network events
	case models.NetworkDeleted:
		return eventLog.Description // Use the custom description for network events
	case models.ScanStarted:
		return eventLog.Description // Use the custom description for scan events
	case models.ScanStopped:
		return eventLog.Description // Use the custom description for scan events
	case models.Warning:
		return "Warning event occurred"
	case models.Alert:
		return "Alert event occurred"
	default:
		return fmt.Sprintf("System event: %s", string(eventLog.Type))
	}
}

func (s *EventLogService) GetAllByDeviceId(deviceId string, limitSize int64) ([]models.EventLog, error) {
	ctx := context.Background()
	eventLogPtrs, err := s.repository.FindAllByDeviceID(ctx, deviceId)
	if err != nil {
		return nil, err
	}

	// Convert pointers to values and respect the limit
	count := int(limitSize)
	if count > len(eventLogPtrs) {
		count = len(eventLogPtrs)
	}
	
	eventLogs := make([]models.EventLog, count)
	for i := 0; i < count; i++ {
		eventLogs[i] = *eventLogPtrs[i]
	}
	
	return eventLogs, nil
}

func (s *EventLogService) CreateOne(eventLog *models.EventLog) error {
	now := time.Now()
	eventLog.CreatedAt = &now
	eventLog.UpdatedAt = &now

	// Use DB manager to serialize database access
	return s.dbManager.CreateEventLog(s.repository, context.Background(), eventLog)
}

func (s *EventLogService) Log(eventType models.EEventLogType, description string, deviceID string) error {
	eventLog := &models.EventLog{
		Type:        eventType,
		Description: description,
	}
	
	if deviceID != "" {
		eventLog.DeviceID = &deviceID
	}
	
	return s.CreateOne(eventLog)
}
