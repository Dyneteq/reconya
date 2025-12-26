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

func (s *NetworkService) Create(name, cidr, description string) (*models.Network, error) {
	now := time.Now()
	network := &models.Network{
		Name:        name,
		CIDR:        cidr,
		Description: description,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	log.Printf("NetworkService.Create: Creating network with CIDR=%s, Name=%s", cidr, name)
	result, err := s.dbManager.CreateOrUpdateNetwork(s.Repository, context.Background(), network)
	if err != nil {
		log.Printf("NetworkService.Create: Error from dbManager: %v", err)
		return nil, err
	}
	log.Printf("NetworkService.Create: Network saved successfully with ID=%s", result.ID)
	return result, nil
}

func (s *NetworkService) FindOrCreate(cidr string) (*models.Network, error) {
	network, err := s.Repository.FindByCIDR(context.Background(), cidr)
	if err == db.ErrNotFound {
		return s.Create("", cidr, "")
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

func (s *NetworkService) Update(id, name, cidr, description string) (*models.Network, error) {
	network, err := s.FindByID(id)
	if err != nil {
		return nil, err
	}
	if network == nil {
		return nil, db.ErrNotFound
	}
	
	network.Name = name
	network.CIDR = cidr
	network.Description = description
	network.UpdatedAt = time.Now()
	
	return s.dbManager.CreateOrUpdateNetwork(s.Repository, context.Background(), network)
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
