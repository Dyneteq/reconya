package network

import (
	"context"
	"log"
	"reconya/db"
	"reconya/internal/config"
	"reconya/models"
	"time"
)

type NetworkService struct {
	Config     *config.Config
	Repository db.NetworkRepository
	dbManager  *db.DBManager
}

func NewNetworkService(networkRepo db.NetworkRepository, cfg *config.Config, dbManager *db.DBManager) *NetworkService {
	return &NetworkService{
		Config:     cfg,
		Repository: networkRepo,
		dbManager:  dbManager,
	}
}

// Create creates a network owning one or more CIDR ranges. labels is optional
// and, if provided, must be the same length as cidrs.
func (s *NetworkService) Create(name string, cidrs []string, labels []string, description string) (*models.Network, error) {
	if err := models.ValidateNetworkRanges(cidrs); err != nil {
		return nil, err
	}

	now := time.Now()
	ranges := buildRanges(cidrs, labels, now)
	network := &models.Network{
		Name:        name,
		CIDR:        ranges[0].CIDR,
		Description: description,
		Status:      "active",
		Ranges:      ranges,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	log.Printf("NetworkService.Create: Creating network with %d range(s), Name=%s", len(cidrs), name)
	result, err := s.dbManager.CreateOrUpdateNetwork(s.Repository, context.Background(), network)
	if err != nil {
		log.Printf("NetworkService.Create: Error from dbManager: %v", err)
		return nil, err
	}
	log.Printf("NetworkService.Create: Network saved successfully with ID=%s", result.ID)
	return result, nil
}

func buildRanges(cidrs []string, labels []string, now time.Time) []models.NetworkRange {
	ranges := make([]models.NetworkRange, len(cidrs))
	for i, cidr := range cidrs {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		ranges[i] = models.NetworkRange{
			CIDR:      cidr,
			Label:     label,
			Active:    true,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return ranges
}

// FindOrCreate is used by network auto-discovery, which only ever deals with
// a single detected subnet.
func (s *NetworkService) FindOrCreate(cidr string) (*models.Network, error) {
	network, err := s.Repository.FindByCIDR(context.Background(), cidr)
	if err == db.ErrNotFound {
		return s.Create("", []string{cidr}, nil, "")
	}
	if err != nil {
		return nil, err
	}
	return network, nil
}

func (s *NetworkService) FindByID(id string) (*models.Network, error) {
	network, err := s.Repository.FindByID(context.Background(), id)
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return network, nil
}

func (s *NetworkService) FindByCIDR(cidr string) (*models.Network, error) {
	network, err := s.Repository.FindByCIDR(context.Background(), cidr)
	if err == db.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return network, nil
}

func (s *NetworkService) FindAll() ([]models.Network, error) {
	log.Printf("NetworkService.FindAll: Fetching all networks")
	networks, err := s.Repository.FindAll(context.Background())
	if err != nil {
		log.Printf("NetworkService.FindAll: Error from repository: %v", err)
		return nil, err
	}

	log.Printf("NetworkService.FindAll: Found %d networks", len(networks))
	result := make([]models.Network, len(networks))
	for i, network := range networks {
		log.Printf("NetworkService.FindAll: Network %d - ID=%s, CIDR=%s, Name=%s", i, network.ID, network.CIDR, network.Name)
		result[i] = *network
	}
	return result, nil
}

// Update replaces a network's name/ranges/description. labels is optional
// and, if provided, must be the same length as cidrs. Existing ranges whose
// CIDR matches one in cidrs keep their id and last_scanned_at history.
func (s *NetworkService) Update(id, name string, cidrs []string, labels []string, description string) (*models.Network, error) {
	if err := models.ValidateNetworkRanges(cidrs); err != nil {
		return nil, err
	}

	network, err := s.FindByID(id)
	if err != nil {
		return nil, err
	}
	if network == nil {
		return nil, db.ErrNotFound
	}

	now := time.Now()
	network.Name = name
	network.Ranges = buildRanges(cidrs, labels, now)
	network.CIDR = network.Ranges[0].CIDR
	network.Description = description
	network.UpdatedAt = now

	return s.dbManager.CreateOrUpdateNetwork(s.Repository, context.Background(), network)
}

// SetRangeActive includes or excludes a range from future scans without
// losing its scan history.
func (s *NetworkService) SetRangeActive(rangeID string, active bool) error {
	return s.Repository.SetRangeActive(context.Background(), rangeID, active)
}

// MarkRangeScanned records when a range was last swept.
func (s *NetworkService) MarkRangeScanned(rangeID string, scannedAt time.Time) error {
	return s.Repository.UpdateRangeLastScanned(context.Background(), rangeID, scannedAt)
}

func (s *NetworkService) Delete(id string) error {
	return s.Repository.Delete(context.Background(), id)
}

func (s *NetworkService) GetDeviceCount(networkID string) (int, error) {
	log.Printf("NetworkService.GetDeviceCount: Counting devices for network %s", networkID)

	count, err := s.Repository.GetDeviceCount(context.Background(), networkID)
	if err != nil {
		log.Printf("NetworkService.GetDeviceCount: Error counting devices: %v", err)
		return 0, err
	}

	log.Printf("NetworkService.GetDeviceCount: Found %d devices for network %s", count, networkID)
	return count, nil
}
