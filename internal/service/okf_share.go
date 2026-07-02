package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// Sentinel errors for the OKF share reader. Callers branch on these to map
// back to HTTP statuses (404 for not-found, 403 for resType mismatch, 400
// for a non-bundle share presented to a bundle endpoint).
var (
	// ErrOkfShareResTypeMismatch is returned when a share code is valid but
	// points to a non-bundle resource (e.g. file share used on a bundle
	// endpoint).
	ErrOkfShareResTypeMismatch = errors.New("okf share: code is not for a bundle")
	// ErrOkfShareBundleMismatch is returned when a share code is for bundle A
	// but the caller asked for bundle B (or node-of-bundle-B).
	ErrOkfShareBundleMismatch = errors.New("okf share: code does not match bundle")
)

// OkfShareReader is the read-only OKF accessor used by public share-code
// endpoints. Unlike OkfService it skips the HybridAuth visibility check —
// possession of a valid (unexpired, under-limit) share code IS the grant.
// Every method still asserts the share points at the bundle/node the caller
// asked for, so a code valid for bundle A cannot read bundle B.
type OkfShareReader struct {
	bundles  okfBundleRepo
	nodes    okfNodeRepo
	edges    okfEdgeRepo
	pdSvc    okfPublicDir
	dbDriver string
}

// NewOkfShareReader constructs an OkfShareReader from the same repos the
// authed OkfService uses. dbDriver selects the per-bundle node-list path
// (MySQL JSON_CONTAINS vs SQLite json_extract) the same way OkfService does.
func NewOkfShareReader(bundles okfBundleRepo, nodes okfNodeRepo, edges okfEdgeRepo, pdSvc okfPublicDir, dbDriver string) *OkfShareReader {
	return &OkfShareReader{bundles: bundles, nodes: nodes, edges: edges, pdSvc: pdSvc, dbDriver: dbDriver}
}

// requireShareBundle centralizes the share-code → bundle cross-check. Returns
// the bundle on success; one of the sentinel errors otherwise.
func (r *OkfShareReader) requireShareBundle(share *model.DiskShare, bundleID uint64) (*model.OkfBundle, error) {
	if share.ResType != "bundle" {
		return nil, ErrOkfShareResTypeMismatch
	}
	if share.ResourceID != bundleID {
		return nil, ErrOkfShareBundleMismatch
	}
	b, err := r.bundles.GetByID(bundleID)
	if err != nil {
		return nil, ErrOkfBundleNotFound
	}
	return b, nil
}

// GetBundle returns the bundle metadata for a share. The share code must be
// for the requested bundleID.
func (r *OkfShareReader) GetBundle(share *model.DiskShare, bundleID uint64) (*model.OkfBundle, error) {
	return r.requireShareBundle(share, bundleID)
}

// ListNodes returns the bundle's nodes, optionally narrowed by type or tag.
func (r *OkfShareReader) ListNodes(share *model.DiskShare, bundleID uint64, typeFilter, tagFilter string) ([]model.OkfNode, error) {
	if _, err := r.requireShareBundle(share, bundleID); err != nil {
		return nil, err
	}
	filter := repository.NodeListFilter{Type: typeFilter, Tag: tagFilter}
	if r.dbDriver == "sqlite" {
		return r.nodes.ListByBundleSQLite(bundleID, filter)
	}
	return r.nodes.ListByBundle(bundleID, filter)
}

// ShareSubgraphRequest is the body shape for the public subgraph endpoint.
// Same fields as the authed SubgraphRequest; the bundleID must match the share.
type ShareSubgraphRequest struct {
	BundleID uint64
	Types    []string
	MaxNodes int
}

// Subgraph returns a type-filtered slice of the bundle graph. Capped by
// maxNodes to keep the share payload bounded.
func (r *OkfShareReader) Subgraph(ctx context.Context, share *model.DiskShare, req ShareSubgraphRequest) (*SubgraphResponse, error) {
	if _, err := r.requireShareBundle(share, req.BundleID); err != nil {
		return nil, err
	}
	// Delegate to OkfService.Subgraph by reconstructing a minimal service.
	// The shared BFS + edge lookup logic is non-trivial; rather than duplicate,
	// we route through a transient OkfService constructed with the same deps.
	// This keeps the read-only share path consistent with the authed one.
	svc := NewOkfServiceFromRepo(r.bundles, r.nodes, r.edges, r.pdSvc, r.dbDriver)
	return svc.Subgraph(ctx, SubgraphRequest{
		BundleID: req.BundleID,
		Types:    req.Types,
		MaxNodes: req.MaxNodes,
	})
}

// ShareNeighborsRequest mirrors the authed NeighborsRequest; nodeID must
// belong to the share's bundle.
type ShareNeighborsRequest struct {
	NodeID     uint64
	Direction  string
	TypeFilter string
}

// Neighbors returns the 1-hop neighbor set of a node. The node must be in
// the bundle the share was created for.
func (r *OkfShareReader) Neighbors(ctx context.Context, share *model.DiskShare, req ShareNeighborsRequest) (*NeighborsResponse, error) {
	node, err := r.nodes.GetByID(req.NodeID)
	if err != nil {
		return nil, ErrOkfNodeNotFound
	}
	if _, err := r.requireShareBundle(share, node.BundleID); err != nil {
		return nil, err
	}
	svc := NewOkfServiceFromRepo(r.bundles, r.nodes, r.edges, r.pdSvc, r.dbDriver)
	return svc.Neighbors(ctx, NeighborsRequest{
		NodeID:     req.NodeID,
		Direction:  req.Direction,
		TypeFilter: req.TypeFilter,
	})
}

// GetNode returns a node + its raw markdown body. Used by the front-end to
// render the markdown preview inside the frontmatter drawer. The body is
// read via pdSvc.ReadFileContent using the node's fileId, so no separate
// preview endpoint is needed.
func (r *OkfShareReader) GetNode(share *model.DiskShare, nodeID uint64) (*model.OkfNode, []byte, error) {
	node, err := r.nodes.GetByID(nodeID)
	if err != nil {
		return nil, nil, ErrOkfNodeNotFound
	}
	if _, bErr := r.requireShareBundle(share, node.BundleID); bErr != nil {
		return nil, nil, bErr
	}
	body, err := r.pdSvc.ReadFileContent(context.Background(), node.FileID)
	if err != nil {
		return nil, nil, fmt.Errorf("read node body: %w", err)
	}
	return node, body, nil
}
