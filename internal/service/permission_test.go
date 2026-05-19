package service

import (
	"fmt"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

type mockPermRepo struct {
	resources map[string]*repository.ResourceOwner // key: "resType:resourceID"
	perms     map[string]*model.DiskPermission     // key: "agentId:resType:resourceID"
}

func newMockPermRepo() *mockPermRepo {
	return &mockPermRepo{
		resources: make(map[string]*repository.ResourceOwner),
		perms:     make(map[string]*model.DiskPermission),
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

func (m *mockPermRepo) GetResourceDetail(resourceID uint64, resType string) (*repository.ResourceOwner, error) {
	r, ok := m.resources[m.key(resType, resourceID)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return r, nil
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
