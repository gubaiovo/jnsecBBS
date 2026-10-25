package auth

import (
	"github.com/exzco/bbs/internal/models"
	"testing"
)

func TestInheritedPermissionsAndSeparateReply(t *testing.T) {
	parent := models.Tag{ID: 10, IsRestricted: true}
	child := models.Tag{ID: 11, ParentID: &parent.ID}
	tp := TagPerms{Tags: map[uint]models.Tag{10: parent}, View: map[uint][]uint{10: {models.GroupMember}}, Post: map[uint][]uint{10: {models.GroupMod}}, Reply: map[uint][]uint{10: {models.GroupMember}}}
	FillTagPerm(nil, &child, tp)
	member := &models.User{ID: 1}
	if !CanViewTag(member, &child) || !CanReplyTag(member, &child) || CanPostTag(member, &child) {
		t.Fatal("parent permissions or separate reply semantics lost")
	}
	if CanViewTag(nil, &child) {
		t.Fatal("guest bypassed restricted parent")
	}
	tp.Failed = true
	FillTagPerm(nil, &child, tp)
	if CanViewTag(member, &child) || CanReplyTag(member, &child) {
		t.Fatal("failed permission load must deny access")
	}
}
