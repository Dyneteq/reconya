package systemstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reconya/db"
	"reconya/models"
	"time"

	"github.com/google/uuid"
)

type SystemStatusService struct {
	repository    db.SystemStatusRepository
	geoRepository *db.GeolocationRepository
}

func NewSystemStatusService(repository db.SystemStatusRepository, geoRepository *db.GeolocationRepository) *SystemStatusService {
	return &SystemStatusService{
		repository:    repository,
		geoRepository: geoRepository,
	}
}

func (s *SystemStatusService) GetLatest() (*models.SystemStatus, error) {
	systemStatus, err := s.repository.FindLatest(context.Background())
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return systemStatus, nil
}

func (s *SystemStatusService) CreateOrUpdate(systemStatus *models.SystemStatus) (*models.SystemStatus, error) {
	now := time.Now()
	systemStatus.CreatedAt = now
	systemStatus.UpdatedAt = now

	// Create the system status
	if err := s.repository.Create(context.Background(), systemStatus); err != nil {
		return nil, err
	}

	// Return the latest system status
	return s.GetLatest()
}

func (s *SystemStatusService) FetchGeolocation(publicIP string) (*models.GeolocationCache, error) {
	if s.geoRepository == nil {
		return nil, nil
	}

	// Try to get from cache first
	geo, err := s.geoRepository.FindByIP(context.Background(), publicIP)
	if err == nil && geo != nil {
		log.Printf("Geolocation cache hit for IP %s", publicIP)
		return geo, nil
	}

	// If not in cache, fetch from API
	log.Printf("Geolocation cache miss for IP %s, fetching from API", publicIP)
	geo, err = s.fetchFromAPI(publicIP)
	if err != nil {
		log.Printf("Failed to fetch geolocation from API for IP %s: %v", publicIP, err)
		return nil, err
	}

	// Save to cache
	if err := s.geoRepository.Create(context.Background(), geo); err != nil {
		log.Printf("Failed to cache geolocation for IP %s: %v", publicIP, err)
	}

	return geo, nil
}

type ipAPIResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
}

func (s *SystemStatusService) fetchFromAPI(publicIP string) (*models.GeolocationCache, error) {
	// Use ip-api.com free API
	url := fmt.Sprintf("http://ip-api.com/json/%s", publicIP)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch geolocation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geolocation API returned status %d", resp.StatusCode)
	}

	var apiResp ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode geolocation response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("geolocation API returned status: %s", apiResp.Status)
	}

	// Create GeolocationCache model
	now := time.Now()
	geo := &models.GeolocationCache{
		ID:          uuid.New().String(),
		IP:          publicIP,
		City:        apiResp.City,
		Region:      apiResp.RegionName,
		Country:     apiResp.Country,
		CountryCode: apiResp.CountryCode,
		Latitude:    apiResp.Lat,
		Longitude:   apiResp.Lon,
		Timezone:    apiResp.Timezone,
		ISP:         apiResp.ISP,
		Source:      "ip-api",
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(30 * 24 * time.Hour), // Cache for 30 days
	}

	log.Printf("Fetched geolocation for IP %s: %s, %s, %s", publicIP, geo.City, geo.Region, geo.Country)
	return geo, nil
}
