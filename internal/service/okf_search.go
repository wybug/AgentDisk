package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"gorm.io/gorm"
)

// SearchRequest is the input to OkfService.Search. BundleID, Type, Limit, and
// Cursor are optional; Query and the caller identity (UserID + Department)
// are required.
type SearchRequest struct {
	Query      string
	BundleID   uint64 // 0 = search every bundle visible to the caller
	Type       string
	Limit      int
	Cursor     uint64
	UserID     string
	Department string
}

// SearchResponse is the paginated search result. NextCursor is 0 when the
// returned page is the last; pass it back as SearchRequest.Cursor for the
// next page.
type SearchResponse struct {
	Nodes      []model.OkfNode `json:"nodes"`
	NextCursor uint64          `json:"nextCursor"`
}

// Search runs a full-text query across the caller's visible bundles. The
// visibility filter is enforced before the repo call so a reader cannot use
// search to discover nodes in a bundle they could not open directly.
//
// When req.BundleID is non-zero the search is scoped to that bundle and the
// service rejects with ErrOkfForbidden if the caller cannot see it. When
// req.BundleID is zero the service enumerates every visible bundle and
// passes the bundle ID list to the repo. A caller with no visible bundles
// gets an empty result rather than an error.
//
// Empty query returns an empty result — the handler is expected to reject
// that case with a 400 before calling in, but the service tolerates it so
// a direct service caller (e.g. a future CLI command) does not crash.
func (s *OkfService) Search(_ context.Context, req SearchRequest) (*SearchResponse, error) {
	if req.Query == "" {
		return &SearchResponse{Nodes: []model.OkfNode{}}, nil
	}
	bundleIDs, err := s.resolveSearchBundles(req)
	if err != nil {
		return nil, err
	}
	filter := repository.SearchFilter{BundleIDs: bundleIDs, Type: req.Type}
	var (
		nodes []model.OkfNode
		next  uint64
	)
	if s.dbDriver == "sqlite" {
		nodes, next, err = s.nodes.SearchSQLite(req.Query, filter, req.Limit, req.Cursor)
	} else {
		nodes, next, err = s.nodes.Search(req.Query, filter, req.Limit, req.Cursor)
	}
	if err != nil {
		return nil, fmt.Errorf("search nodes: %w", err)
	}
	if nodes == nil {
		nodes = []model.OkfNode{}
	}
	return &SearchResponse{Nodes: nodes, NextCursor: next}, nil
}

// resolveSearchBundles returns the set of bundle IDs the caller is allowed
// to search. When req.BundleID is set, it is validated against the visible
// set (or returned directly when no visibility provider is configured);
// otherwise every visible bundle's ID is included.
//
// Returns an empty slice (not nil) when the caller has no visible bundles,
// so the repo layer's `bundle_id IN ?` clause can be skipped — passing nil
// to IN is an error on some drivers.
func (s *OkfService) resolveSearchBundles(req SearchRequest) ([]uint64, error) {
	if req.BundleID != 0 {
		bundle, err := s.bundles.GetByID(req.BundleID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrOkfBundleNotFound
			}
			return nil, fmt.Errorf("lookup bundle: %w", err)
		}
		if err := s.requireBundleVisible(bundle, req.UserID, req.Department); err != nil {
			return nil, err
		}
		return []uint64{req.BundleID}, nil
	}
	all, err := s.bundles.List("", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("list bundles: %w", err)
	}
	allowed, err := s.visibleBundleSet(req.UserID, req.Department)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(all))
	for i := range all {
		if allowed == nil || allowed[all[i].PublicDirectoryID] {
			out = append(out, all[i].ID)
		}
	}
	return out, nil
}
