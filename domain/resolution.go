package domain

import "fmt"

// CapabilityResolutionService resolves required capabilities from the registry.
type CapabilityResolutionService struct {
	capabilityRepo CapabilityRepository
}

// NewCapabilityResolutionService creates a CapabilityResolutionService.
func NewCapabilityResolutionService(capabilityRepo CapabilityRepository) *CapabilityResolutionService {
	return &CapabilityResolutionService{capabilityRepo: capabilityRepo}
}

// Resolve looks up all required capabilities and returns their definitions.
func (s *CapabilityResolutionService) Resolve(requirements RequirementSet) ([]*CapabilityDefinition, error) {
	resolved := make([]*CapabilityDefinition, 0, len(requirements))
	for _, req := range requirements {
		cap, err := s.capabilityRepo.GetByName(req.Capability)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve capability %q: %w", req.Capability, err)
		}
		if cap == nil {
			return nil, fmt.Errorf("required capability %q not found", req.Capability)
		}
		resolved = append(resolved, cap)
	}
	return resolved, nil
}
