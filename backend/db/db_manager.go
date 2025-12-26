package db

import (
	"context"
	"log"
	"reconya/models"
	"time"
)

// Operation represents a database operation that needs to be executed
type Operation struct {
	Execute func() error
	Result  chan error
}

// OperationWithResult represents a database operation that returns a result
type OperationWithResult struct {
	Execute func() (interface{}, error)
	Result  chan OperationResult
}

// OperationResult contains the result of an operation
type OperationResult struct {
	Data  interface{}
	Error error
}

// DBManager manages serialized access to the database
type DBManager struct {
	opQueue      chan Operation
	resultOpQueue chan OperationWithResult
	stopping     chan struct{}
}

// NewDBManager creates a new database manager
func NewDBManager() *DBManager {
	m := &DBManager{
		opQueue:      make(chan Operation, 100),
		resultOpQueue: make(chan OperationWithResult, 100),
		stopping:     make(chan struct{}),
	}

	// Start the worker goroutine
	go m.worker()
	log.Println("Database access manager started")

	return m
}

// worker processes operations one at a time
func (m *DBManager) worker() {
	for {
		select {
		case op := <-m.opQueue:
			err := op.Execute()
			op.Result <- err
		case op := <-m.resultOpQueue:
			data, err := op.Execute()
			op.Result <- OperationResult{Data: data, Error: err}
		case <-m.stopping:
			return
		}
	}
}

// ExecuteOperation executes a database operation with retries
func (m *DBManager) ExecuteOperation(execute func() error) error {
	resultChan := make(chan error, 1)
	m.opQueue <- Operation{
		Execute: execute,
		Result:  resultChan,
	}
	return <-resultChan
}

// ExecuteOperationWithResult executes a database operation that returns a result with retries
func (m *DBManager) ExecuteOperationWithResult(execute func() (interface{}, error)) (interface{}, error) {
	resultChan := make(chan OperationResult, 1)
	m.resultOpQueue <- OperationWithResult{
		Execute: execute,
		Result:  resultChan,
	}
	result := <-resultChan
	return result.Data, result.Error
}

// Stop stops the database manager
func (m *DBManager) Stop() {
	close(m.stopping)
}

// Methods for specific repository operations

// CreateOrUpdateDevice serializes access to device creation/updates
func (m *DBManager) CreateOrUpdateDevice(repo DeviceRepository, ctx context.Context, device *models.Device) (*models.Device, error) {
	result, err := m.ExecuteOperationWithResult(func() (interface{}, error) {
		return repo.CreateOrUpdate(ctx, device)
	})
	if err != nil {
		return nil, err
	}
	return result.(*models.Device), nil
}

// UpdateDeviceStatuses serializes access to device status updates
func (m *DBManager) UpdateDeviceStatuses(repo DeviceRepository, ctx context.Context, timeout time.Duration) error {
	return m.ExecuteOperation(func() error {
		return repo.UpdateDeviceStatuses(ctx, timeout)
	})
}

// CreateEventLog serializes access to event log creation
func (m *DBManager) CreateEventLog(repo EventLogRepository, ctx context.Context, eventLog *models.EventLog) error {
	return m.ExecuteOperation(func() error {
		return repo.Create(ctx, eventLog)
	})
}

// CreateOrUpdateNetwork serializes access to network creation/updates
func (m *DBManager) CreateOrUpdateNetwork(repo NetworkRepository, ctx context.Context, network *models.Network) (*models.Network, error) {
	result, err := m.ExecuteOperationWithResult(func() (interface{}, error) {
		return repo.CreateOrUpdate(ctx, network)
	})
	if err != nil {
		return nil, err
	}
	return result.(*models.Network), nil
}

