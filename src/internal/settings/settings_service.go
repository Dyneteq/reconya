package settings

import (
	"errors"
	"log"
	"reconya/db"
	"reconya/models"
	"time"

	"github.com/google/uuid"
)

// SettingsService handles settings operations
type SettingsService struct {
	repo db.SettingsRepository
}

// NewSettingsService creates a new settings service
func NewSettingsService(repo db.SettingsRepository) *SettingsService {
	return &SettingsService{
		repo: repo,
	}
}

// GetUserSettings retrieves settings for a specific user
func (s *SettingsService) GetUserSettings(userID string) (*models.Settings, error) {
	settings, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}
	
	// If no settings exist for user, create default settings
	if settings == nil {
		settings = models.DefaultSettings()
		settings.UserID = userID
		settings.ID = uuid.New().String()
		now := time.Now()
		settings.CreatedAt = &now
		settings.UpdatedAt = &now
		
		// Save default settings to database
		err = s.repo.Create(settings)
		if err != nil {
			log.Printf("Error creating default settings for user %s: %v", userID, err)
			return settings, nil // Return defaults even if save fails
		}
	}
	
	return settings, nil
}

// UpdateUserSettings updates settings for a specific user
func (s *SettingsService) UpdateUserSettings(userID string, updates map[string]interface{}) (*models.Settings, error) {
	// Get current settings
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		return nil, err
	}
	
	// Apply updates
	for key, value := range updates {
		switch key {
		case "screenshots_enabled":
			if boolVal, ok := value.(bool); ok {
				settings.ScreenshotsEnabled = boolVal
			} else {
				return nil, errors.New("screenshots_enabled must be a boolean")
			}
		default:
			return nil, errors.New("unknown setting: " + key)
		}
	}
	
	// Update timestamp
	now := time.Now()
	settings.UpdatedAt = &now
	
	// Save to database
	err = s.repo.Update(settings)
	if err != nil {
		return nil, err
	}
	
	return settings, nil
}

// AreScreenshotsEnabled checks if screenshots are enabled for a user
func (s *SettingsService) AreScreenshotsEnabled(userID string) bool {
	settings, err := s.GetUserSettings(userID)
	if err != nil {
		log.Printf("Error getting settings for user %s: %v", userID, err)
		return true // Default to enabled if error occurs
	}
	
	return settings.ScreenshotsEnabled
}