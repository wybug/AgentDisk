package model

import (
	"encoding/json"
	"errors"
	"time"
)

// OkfBundleStatusActive and OkfBundleStatusArchived are the allowed values for
// OkfBundle.Status. Strings (not a typed enum) keep the column forward-
// compatible: new statuses can be introduced without a schema migration, and
// readers stay tolerant of unknown values per OKF §9.
const (
	OkfBundleStatusActive   = "active"
	OkfBundleStatusArchived = "archived"
	OkfBundleDefaultVersion = "0.1"
	OkfReservedRootRelPath  = "index.md"
	OkfReservedLogRelPath   = "log.md"
)

// OkfBundle represents a registered OKF bundle. A bundle maps 1:1 to a public
// directory: the public directory supplies the folder tree and OSS storage,
// while the bundle holds OKF-specific metadata and a denormalized node count.
type OkfBundle struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PublicDirectoryID uint64    `gorm:"uniqueIndex;not null" json:"publicDirectoryId"`
	OkfVersion        string    `gorm:"size:16;not null;default:0.1" json:"okfVersion"`
	RootIndexFileID   uint64    `gorm:"not null;default:0" json:"rootIndexFileId"`
	Title             string    `gorm:"size:255" json:"title"`
	Description       string    `gorm:"type:text" json:"description"`
	Status            string    `gorm:"size:16;not null;default:active" json:"status"`
	NodeCount         uint32    `gorm:"not null;default:0" json:"nodeCount"`
	EdgeCount         uint32    `gorm:"not null;default:0" json:"edgeCount"`
	CreatedAt         time.Time `gorm:"type:datetime;not null" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"type:datetime;not null" json:"updatedAt"`
}

// TableName overrides the table name for OkfBundle.
func (OkfBundle) TableName() string { return "disk_okf_bundle" }

// OkfNode represents one materialized OKF markdown document inside a bundle.
// Frontmatter scalar fields are promoted to columns for indexing; complex or
// unknown fields are serialized into TagsJSON / ExtraJSON so the schema stays
// forward-compatible (OKF §9 tolerance).
type OkfNode struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	BundleID      uint64    `gorm:"uniqueIndex:uk_bundle_rel_path,priority:1;not null" json:"bundleId"`
	FileID        uint64    `gorm:"index:idx_bundle_file,priority:1;not null" json:"fileId"`
	RelPath       string    `gorm:"size:1024;uniqueIndex:uk_bundle_rel_path,priority:2;not null" json:"relPath"`
	Type          string    `gorm:"size:64;not null;default:'';index:idx_bundle_type,priority:2" json:"type"`
	Title         string    `gorm:"size:255" json:"title"`
	Description   string    `gorm:"type:text" json:"description"`
	TagsJSON      *string   `gorm:"type:json" json:"tagsJson,omitempty"`
	Timestamp     string    `gorm:"size:32" json:"timestamp"`
	HasBrokenLink bool      `gorm:"not null;default:false" json:"hasBrokenLink"`
	LinkCount     uint32    `gorm:"not null;default:0" json:"linkCount"`
	BacklinkCount uint32    `gorm:"not null;default:0" json:"backlinkCount"`
	ExtraJSON     *string   `gorm:"type:json" json:"extraJson,omitempty"`
	ContentHash   string    `gorm:"size:64;index:idx_content_hash" json:"contentHash"`
	CreatedAt     time.Time `gorm:"type:datetime;not null" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"type:datetime;not null" json:"updatedAt"`
}

// TableName overrides the table name for OkfNode.
func (OkfNode) TableName() string { return "disk_okf_node" }

// OkfEdge is one directed link between two OkfNodes (or to a missing dst,
// when DstExists=false). The edge table is the materialized knowledge graph:
// writes to a bundle's .md files upsert the source node's edges atomically.
//
// All queries carry PublicDirID so a bundle's graph slice is clustered on a
// composite index — that is what makes 1-hop and reverse traversal cheap. The
// unique key on (src_node_id, dst_rel_path, src_line) makes edge replacement
// idempotent: re-writing the same .md produces the same edge set.
type OkfEdge struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PublicDirID uint64    `gorm:"not null;index:idx_edge_src,priority:1;index:idx_edge_dst,priority:1;index:idx_edge_broken,priority:1;index:idx_edge_relpath,priority:1" json:"publicDirId"`
	SrcNodeID   uint64    `gorm:"not null;uniqueIndex:uk_edge_src_line,priority:1;index:idx_edge_src,priority:2" json:"srcNodeId"`
	DstNodeID   uint64    `gorm:"index:idx_edge_dst,priority:2" json:"dstNodeId"`
	DstRelPath  string    `gorm:"size:1024;not null;uniqueIndex:uk_edge_src_line,priority:2" json:"dstRelPath"`
	DstExists   bool      `gorm:"not null;default:false;index:idx_edge_broken,priority:2" json:"dstExists"`
	LinkText    string    `gorm:"size:512" json:"linkText"`
	SrcLine     int       `gorm:"not null;default:0;uniqueIndex:uk_edge_src_line,priority:3" json:"srcLine"`
	LinkKind    string    `gorm:"size:16;not null" json:"linkKind"`
	Anchor      string    `gorm:"size:128" json:"anchor"`
	CreatedAt   time.Time `gorm:"type:datetime;not null" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"type:datetime;not null" json:"updatedAt"`
}

// TableName overrides the table name for OkfEdge.
func (OkfEdge) TableName() string { return "disk_okf_edge" }

// GetTags deserializes TagsJSON into a []string. An empty or missing JSON
// value yields a non-nil empty slice so callers can range without a nil check.
// A corrupt JSON value yields an empty slice and the caller receives the
// parse error so it can decide whether to log it; the node remains readable
// per the OKF §9 tolerance rule.
func (n *OkfNode) GetTags() ([]string, error) {
	if n == nil || n.TagsJSON == nil || *n.TagsJSON == "" {
		return []string{}, nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(*n.TagsJSON), &tags); err != nil {
		return []string{}, err
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}

// SetTags serializes tags into TagsJSON. A nil or empty slice clears the
// field (sets TagsJSON to nil) so the column reads as NULL rather than "[]".
func (n *OkfNode) SetTags(tags []string) error {
	if n == nil {
		return errors.New("okf: nil node")
	}
	if len(tags) == 0 {
		n.TagsJSON = nil
		return nil
	}
	raw, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	s := string(raw)
	n.TagsJSON = &s
	return nil
}

// GetExtra deserializes ExtraJSON into a map[string]any. Missing or empty
// JSON yields a non-nil empty map; a corrupt value yields an empty map and the
// error so callers can decide. Returning an empty map (rather than nil) keeps
// the API friendly for JSON responders that range over the result.
func (n *OkfNode) GetExtra() (map[string]any, error) {
	if n == nil || n.ExtraJSON == nil || *n.ExtraJSON == "" {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(*n.ExtraJSON), &out); err != nil {
		return map[string]any{}, err
	}
	return out, nil
}

// SetExtra serializes extra into ExtraJSON. A nil or empty map clears the
// field so the column reads as NULL rather than "{}".
func (n *OkfNode) SetExtra(extra map[string]any) error {
	if n == nil {
		return errors.New("okf: nil node")
	}
	if len(extra) == 0 {
		n.ExtraJSON = nil
		return nil
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	s := string(raw)
	n.ExtraJSON = &s
	return nil
}
