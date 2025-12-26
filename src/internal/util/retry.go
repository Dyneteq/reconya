package util

import (
	"log"
	"strings"
	"time"
)

// RetryOnLock retries the given function if it fails with a database lock error
func RetryOnLock(operation func() error) error {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond
	
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "database is locked") {
			delay := baseDelay * time.Duration(1<<i)
			log.Printf("Database locked, retrying in %v...", delay)
			time.Sleep(delay)
			continue
		}

		return err
	}

	return err
}

func RetryOnLockWithResult[T any](operation func() (T, error)) (T, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	var result T
	var err error

	for i := 0; i < maxRetries; i++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		if strings.Contains(err.Error(), "database is locked") {
			delay := baseDelay * time.Duration(1<<i)
			log.Printf("Database locked, retrying in %v...", delay)
			time.Sleep(delay)
			continue
		}

		return result, err
	}

	return result, err
}

