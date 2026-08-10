package link

import (
	"encoding/json"
	"testing"
)

func TestQaEnabledForLink(t *testing.T) {
	if QaEnabledForLink(false) {
		t.Fatal("document share links should not enable visitor Q&A")
	}
	if !QaEnabledForLink(true) {
		t.Fatal("deal-room share links should enable visitor Q&A")
	}
}

func TestResolveQaEnabled(t *testing.T) {
	if ResolveQaEnabled(false, true) {
		t.Fatal("document share links should ignore requested qa_enabled")
	}
	if !ResolveQaEnabled(true, true) {
		t.Fatal("deal-room links should always enable qa_enabled")
	}
	if !ResolveQaEnabled(true, false) {
		t.Fatal("deal-room links should ignore qa_enabled=false and stay true")
	}
}

func TestResolveQaEnabledFromOptional(t *testing.T) {
	if ResolveQaEnabledFromOptional(false, nil) {
		t.Fatal("document links should disable Ask")
	}
	falseVal := false
	if ResolveQaEnabledFromOptional(false, &falseVal) {
		t.Fatal("document links should ignore explicit qa_enabled")
	}
	if !ResolveQaEnabledFromOptional(true, nil) {
		t.Fatal("deal-room links should default qa_enabled to true when omitted")
	}
	if !ResolveQaEnabledFromOptional(true, boolPtr(true)) {
		t.Fatal("deal-room links should always enable qa_enabled")
	}
	if !ResolveQaEnabledFromOptional(true, boolPtr(false)) {
		t.Fatal("deal-room links should ignore explicit qa_enabled=false")
	}
}

func boolPtr(v bool) *bool { return &v }

func TestUpdateRequestJSONQaEnabledFalse(t *testing.T) {
	type updateRequest struct {
		QaEnabled *bool `json:"qa_enabled,omitempty"`
	}
	var req updateRequest
	if err := json.Unmarshal([]byte(`{"qa_enabled":false}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.QaEnabled == nil {
		t.Fatal("expected non-nil *bool for qa_enabled:false")
	}
	if *req.QaEnabled {
		t.Fatal("expected false")
	}
}

func TestCreateRequestJSONQaEnabledOmitted(t *testing.T) {
	type createRequest struct {
		DealRoomID string `json:"deal_room_id,omitempty"`
		QaEnabled  *bool  `json:"qa_enabled,omitempty"`
	}
	var req createRequest
	if err := json.Unmarshal([]byte(`{"deal_room_id":"room-1"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.QaEnabled != nil {
		t.Fatal("omitted qa_enabled must unmarshal as nil")
	}
	if !ResolveQaEnabledFromOptional(true, req.QaEnabled) {
		t.Fatal("deal-room create with omitted qa_enabled must default to true")
	}
}

func TestUpdateLinkHandlerQaEnabledDealRoom(t *testing.T) {
	existingTrue := true
	reqFalse := false
	reqNil := (*bool)(nil)

	resolve := func(existingDealRoom bool, existingQa bool, req *bool) bool {
		return existingDealRoom
	}

	if !resolve(true, existingTrue, &reqFalse) {
		t.Fatal("deal-room update must keep qa_enabled true even when client sends false")
	}
	if !resolve(true, existingTrue, reqNil) {
		t.Fatal("deal-room update must keep qa_enabled true when omitted")
	}
	if got := resolve(false, existingTrue, &reqFalse); got {
		t.Fatal("document links must force qa_enabled false")
	}
}
