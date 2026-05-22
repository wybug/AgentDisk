package service

import (
	"fmt"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

type mockPermRepo struct {
	resources     map[string]*repository.ResourceOwner // key: "resType:resourceID"
	perms         map[string]*model.DiskPermission     // key: "agentId:resType:resourceID"
	pathPerms     []model.DiskPermission
	groupPerms    []model.DiskPermission
	resourcePaths map[string]string // key: "resType:resourceID"
}

func newMockPermRepo() *mockPermRepo {
	return &mockPermRepo{
		resources:     make(map[string]*repository.ResourceOwner),
		perms:         make(map[string]*model.DiskPermission),
		resourcePaths: make(map[string]string),
	}
}

func (m *mockPermRepo) key(resType string, resourceID uint64) string {
	return fmt.Sprintf("%s:%d", resType, resourceID)
}

func (m *mockPermRepo) permKey(agentID, resType string, resourceID uint64) string {
	return fmt.Sprintf("%s:%s:%d", agentID, resType, resourceID)
}

func (m *mockPermRepo) addResource(id uint64, resType, owner, sourceAgent, sourceAgentGroup string, isArtifact bool) {
	m.resources[m.key(resType, id)] = &repository.ResourceOwner{
		OwnerID:          owner,
		SourceAgent:      sourceAgent,
		SourceAgentGroup: sourceAgentGroup,
		IsArtifact:       isArtifact,
	}
}

func (m *mockPermRepo) addPerm(agentID string, resourceID uint64, resType, perm string) {
	m.perms[m.permKey(agentID, resType, resourceID)] = &model.DiskPermission{
		AgentID:    agentID,
		ResourceID: resourceID,
		ResType:    resType,
		Permission: perm,
	}
}

func (m *mockPermRepo) addPathPerm(agentID, agentGroupID, resourcePath, perm string) {
	m.pathPerms = append(m.pathPerms, model.DiskPermission{
		AgentID:      agentID,
		AgentGroupID: agentGroupID,
		ResourcePath: resourcePath,
		Permission:   perm,
	})
}

func (m *mockPermRepo) setResourcePath(resourceID uint64, resType, fullPath string) {
	m.resourcePaths[m.key(resType, resourceID)] = fullPath
}

func (m *mockPermRepo) Create(_ *model.DiskPermission) error { return nil }
func (m *mockPermRepo) ListByUser(_ string) ([]model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermRepo) Delete(_ uint64) error { return nil }

func (m *mockPermRepo) GetByAgentAndResource(agentID string, resourceID uint64, resType string) (*model.DiskPermission, error) {
	p, ok := m.perms[m.permKey(agentID, resType, resourceID)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return p, nil
}

func (m *mockPermRepo) GetByAgentAndResourcePath(agentID, resourcePath string) (*model.DiskPermission, error) {
	for i := range m.pathPerms {
		if m.pathPerms[i].AgentID == agentID && m.pathPerms[i].ResourcePath == resourcePath {
			return &m.pathPerms[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPermRepo) GetByGroupAndResource(agentGroupID string, resourceID uint64, resType string) (*model.DiskPermission, error) {
	for i := range m.groupPerms {
		if m.groupPerms[i].AgentGroupID == agentGroupID && m.groupPerms[i].ResourceID == resourceID && m.groupPerms[i].ResType == resType {
			return &m.groupPerms[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPermRepo) GetByGroupAndResourcePath(agentGroupID, resourcePath string) (*model.DiskPermission, error) {
	for i := range m.pathPerms {
		if m.pathPerms[i].AgentGroupID == agentGroupID && m.pathPerms[i].ResourcePath == resourcePath {
			return &m.pathPerms[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPermRepo) GetResourceDetail(resourceID uint64, resType string) (*repository.ResourceOwner, error) {
	r, ok := m.resources[m.key(resType, resourceID)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return r, nil
}

func (m *mockPermRepo) GetResourcePath(resourceID uint64, resType string) (string, error) {
	p, ok := m.resourcePaths[m.key(resType, resourceID)]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return p, nil
}

func (m *mockPermRepo) ListPathPermissionsByAgent(agentID string) ([]model.DiskPermission, error) {
	var result []model.DiskPermission
	for _, p := range m.pathPerms {
		if p.AgentID == agentID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPermRepo) ListGroupPermissions(agentGroupID string) ([]model.DiskPermission, error) {
	var result []model.DiskPermission
	for _, p := range m.pathPerms {
		if p.AgentGroupID == agentGroupID {
			result = append(result, p)
		}
	}
	for _, p := range m.groupPerms {
		if p.AgentGroupID == agentGroupID {
			result = append(result, p)
		}
	}
	return result, nil
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		actual   string
		required string
		expected bool
	}{
		{"owner", "read", true},
		{"owner", "write", true},
		{"owner", "delete", true},
		{"delete", "read", true},
		{"delete", "write", true},
		{"write", "read", true},
		{"write", "delete", false},
		{"read", "write", false},
		{"read", "delete", false},
		{"read", "read", true},
	}
	for _, tt := range tests {
		result := hasPermission(tt.actual, tt.required)
		if result != tt.expected {
			t.Errorf("hasPermission(%s, %s) = %v, expected %v", tt.actual, tt.required, result, tt.expected)
		}
	}
}

func TestCheckOrAutoGrant_UserRequest_AlwaysAllow(t *testing.T) {
	svc := &PermissionService{repo: newMockPermRepo()}
	ok, err := svc.CheckOrAutoGrant("user001", "", "", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("user request should always pass, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_AgentOwnFile_AutoReadWrite(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("agent own file read: ok=%v err=%v", ok, err)
	}
	ok, err = svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "write")
	if err != nil || !ok {
		t.Errorf("agent own file write: ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_SameGroupAgent_AutoReadWrite(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-02", "group-a", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("same group agent read: ok=%v err=%v", ok, err)
	}
	ok, err = svc.CheckOrAutoGrant("user001", "agent-02", "group-a", 1, "file", "write")
	if err != nil || !ok {
		t.Errorf("same group agent write: ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_DifferentGroupAgent_Deny(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-02", "group-b", 1, "file", "read")
	if ok {
		t.Error("different group agent should be denied (no explicit perm)")
	}
}

func TestCheckOrAutoGrant_NonArtifactFile_RequireExplicit(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "read")
	if ok {
		t.Error("non-artifact file should require explicit permission")
	}
}

func TestCheckOrAutoGrant_NonArtifactFile_ExplicitGrant(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.addPerm("agent-01", 1, "file", "read")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("explicit read grant should pass, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_AgentDeletePermission_RequireExplicit(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "delete")
	if ok {
		t.Error("delete permission should require explicit grant even for own file")
	}
}

func TestCheckOrAutoGrant_AgentDeletePermission_ExplicitGrant(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	repo.addPerm("agent-01", 1, "file", "delete")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-01", "group-a", 1, "file", "delete")
	if err != nil || !ok {
		t.Errorf("explicit delete grant should pass, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_CrossUser_AlwaysDeny(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "agent-01", "group-a", true)
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user002", "agent-01", "group-a", 1, "file", "read")
	if err != nil || ok {
		t.Errorf("cross-user should be denied, got ok=%v err=%v", ok, err)
	}
}

// --- Path-based permission tests ---

func TestCheckOrAutoGrant_PathPermission_MatchRead(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("agent-02", "", "/**", "read")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("path /** should grant read, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_PathPermission_ExtensionFilter(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("agent-02", "", "/**/*.txt", "read")
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "read")
	if ok {
		t.Error("/**/*.txt should not match .pdf file")
	}
}

func TestCheckOrAutoGrant_PathPermission_SubfolderOnly(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Downloads/file.pdf")
	repo.addPathPerm("agent-02", "", "/Documents/**", "read")
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "read")
	if ok {
		t.Error("/Documents/** should not match /Downloads/file.pdf")
	}
}

func TestCheckOrAutoGrant_GroupPathPermission(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("", "group-a", "/Documents/**", "read")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-02", "group-a", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("group path permission should grant read, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_PathPermission_WriteNotAllowed(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("agent-02", "", "/**", "read")
	svc := &PermissionService{repo: repo}

	ok, _ := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "write")
	if ok {
		t.Error("read-only path permission should not allow write")
	}
}

func TestCheckOrAutoGrant_ResourceIDAndPathCoexist(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	// No resource ID perm, but path perm exists
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("agent-02", "", "/Documents/**", "read")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("path permission should grant access when no resource ID perm exists, got ok=%v err=%v", ok, err)
	}
}

func TestCheckOrAutoGrant_PathPermission_ExactPath(t *testing.T) {
	repo := newMockPermRepo()
	repo.addResource(1, "file", "user001", "", "", false)
	repo.setResourcePath(1, "file", "/Documents/report.pdf")
	repo.addPathPerm("agent-02", "", "/Documents/report.pdf", "read")
	svc := &PermissionService{repo: repo}

	ok, err := svc.CheckOrAutoGrant("user001", "agent-02", "", 1, "file", "read")
	if err != nil || !ok {
		t.Errorf("exact path should match, got ok=%v err=%v", ok, err)
	}
}

func TestGrantPermission_PathMode(t *testing.T) {
	repo := newMockPermRepo()
	svc := &PermissionService{repo: repo}
	err := svc.GrantPermission("user001", "agent-01", "", 0, "", "/**", "read")
	if err != nil {
		t.Errorf("GrantPermission with path should succeed, got err=%v", err)
	}
}

func TestGrantPermission_GroupMode(t *testing.T) {
	repo := newMockPermRepo()
	svc := &PermissionService{repo: repo}
	err := svc.GrantPermission("user001", "", "group-a", 0, "", "/Documents/**", "write")
	if err != nil {
		t.Errorf("GrantPermission with group should succeed, got err=%v", err)
	}
}
