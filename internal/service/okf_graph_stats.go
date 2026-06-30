package service

import (
	"context"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/repository"
)

// StatsRequest is the input shape for the per-bundle graph stats query.
type StatsRequest struct {
	BundleID   uint64
	UserID     string
	Department string
}

// StatsResponse summarizes a bundle's graph at a point in time. Live + Broken
// = Total edges; NodeCount is the materialized node total; TypeCounts breaks
// nodes down by OKF type.
type StatsResponse struct {
	NodeCount  uint32
	EdgeTotal  uint32
	EdgeLive   uint32
	EdgeBroken uint32
	TypeCounts []repository.TypeCount
}

// Stats returns a per-bundle summary. The NodeCount + TypeCounts come from
// the node repo (reusing the existing aggregate query); EdgeTotal/Live/Broken
// come from the new StatsByBundle repo method. The values are point-in-time
// reads — caching is intentionally skipped because the stats endpoint is
// cheap (one GROUP BY on each table) and stale stats are worse than a slow
// response when debugging a broken bundle.
//
// ACL: the bundle must be visible.
func (s *OkfService) Stats(_ context.Context, req StatsRequest) (*StatsResponse, error) {
	bundle, err := s.bundles.GetByID(req.BundleID)
	if err != nil {
		return nil, ErrOkfBundleNotFound
	}
	if visErr := s.requireBundleVisible(bundle, req.UserID, req.Department); visErr != nil {
		return nil, visErr
	}

	stats, err := s.edges.StatsByBundle(bundle.ID, bundle.PublicDirectoryID)
	if err != nil {
		return nil, fmt.Errorf("edge stats: %w", err)
	}
	typeCounts, err := s.nodes.AggregateByType(bundle.ID)
	if err != nil {
		return nil, fmt.Errorf("type aggregate: %w", err)
	}
	return &StatsResponse{
		NodeCount:  bundle.NodeCount,
		EdgeTotal:  stats.Total,
		EdgeLive:   stats.Live,
		EdgeBroken: stats.Broken,
		TypeCounts: typeCounts,
	}, nil
}
