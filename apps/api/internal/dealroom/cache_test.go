package dealroom

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

type memoryListCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemoryListCache() *memoryListCache {
	return &memoryListCache{data: make(map[string][]byte)}
}

func (c *memoryListCache) Get(_ context.Context, key string, dest interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, ok := c.data[key]
	if !ok {
		return errors.New("cache miss")
	}
	return json.Unmarshal(raw, dest)
}

func (c *memoryListCache) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = raw
	return nil
}

func (c *memoryListCache) Del(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.data, key)
	}
	return nil
}

func TestListRoomsUsesAndInvalidatesSlimCache(t *testing.T) {
	fake := newFakeDB(t)
	cache := newMemoryListCache()
	svc := NewService(db.New(fake), nil, testCfg(), WithListCache(cache))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Cache Workspace",
		Slug:     "cache-workspace",
	}

	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "cache-room",
		Name: "Cache Room",
	}); err != nil {
		t.Fatalf("create room: %v", err)
	}

	first, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 room, got %d", len(first))
	}
	if first[0].Room.Name != "Cache Room" {
		t.Fatalf("unexpected room name %q", first[0].Room.Name)
	}

	fake.rooms = nil
	second, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("cached list rooms: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected cached list to return 1 room, got %d", len(second))
	}
	// Slim cache must not retain settings JSON on cache hits.
	if len(second[0].Room.Settings) != 0 {
		t.Fatalf("cached list payload should not carry settings, got %d bytes", len(second[0].Room.Settings))
	}

	svc.invalidateListCache(context.Background(), wsID)
	third, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("list after invalidate: %v", err)
	}
	if len(third) != 0 {
		t.Fatalf("expected empty list after invalidate, got %d", len(third))
	}
}

func TestListRoomsPageSlicesCachedList(t *testing.T) {
	fake := newFakeDB(t)
	cache := newMemoryListCache()
	svc := NewService(db.New(fake), nil, testCfg(), WithListCache(cache))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Page Workspace",
		Slug:     "page-workspace",
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
			Slug: "room-" + uuid.NewString()[:8],
			Name: "Room",
		}); err != nil {
			t.Fatalf("create room: %v", err)
		}
	}

	page, err := svc.ListRoomsPage(context.Background(), wsID, 1, 2, "")
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("total = %d, want 3", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	if !page.HasMore {
		t.Fatal("expected has_more")
	}
}

func TestListRoomsForUserFiltersCachedWorkspaceList(t *testing.T) {
	t.Setenv("ROOM_LIST_SCOPED", "1")
	fake := newFakeDB(t)
	cache := newMemoryListCache()
	svc := NewService(db.New(fake), nil, testCfg(), WithListCache(cache))
	ownerID := uuid.NewString()
	memberID := uuid.NewString()
	outsiderID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Scoped Cache Workspace",
		Slug:     "scoped-cache-workspace",
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(memberID), Role: "member", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(outsiderID), Role: "member", JoinedAt: nowTs()},
	}

	visible, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "visible-room",
		Name: "Visible Room",
	})
	if err != nil {
		t.Fatalf("create visible room: %v", err)
	}
	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "hidden-room",
		Name: "Hidden Room",
	}); err != nil {
		t.Fatalf("create hidden room: %v", err)
	}

	fake.members = append(fake.members, db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		RoomID:      visible.ID,
		Email:       "member@example.com",
		UserID:      pgUUID(memberID),
		Role:        "guest",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})

	all, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("workspace cache must hold 2 rooms, got %d", len(all))
	}

	leaked, err := svc.ListRoomsForUser(context.Background(), wsID, outsiderID)
	if err != nil {
		t.Fatalf("list for unjoined member: %v", err)
	}
	if len(leaked) != 0 {
		t.Fatalf("cached full list must not leak unjoined rooms, got %d", len(leaked))
	}

	scoped, err := svc.ListRoomsForUser(context.Background(), wsID, memberID)
	if err != nil {
		t.Fatalf("list for invited member: %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("invited member should see 1 room, got %d", len(scoped))
	}
	if scoped[0].Room.ID.Bytes != visible.ID.Bytes {
		t.Fatal("invited member listed unexpected room")
	}

	oversight, err := svc.ListRoomsForUser(context.Background(), wsID, ownerID)
	if err != nil {
		t.Fatalf("list for workspace owner: %v", err)
	}
	if len(oversight) != 2 {
		t.Fatalf("workspace owner should see cached full list, got %d", len(oversight))
	}
}

func TestListCacheKeyStable(t *testing.T) {
	ws := uuid.NewString()
	if listCacheKey(ws) != "dealrooms:list:v8:"+ws {
		t.Fatalf("unexpected cache key: %s", listCacheKey(ws))
	}
}

type debounceListCache struct {
	*memoryListCache
	acquired map[string]bool
}

func (c *debounceListCache) TryAcquireDebounce(_ context.Context, key string, _ time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.acquired == nil {
		c.acquired = make(map[string]bool)
	}
	if c.acquired[key] {
		return false
	}
	c.acquired[key] = true
	return true
}

func TestSoftInvalidateListCacheDebounces(t *testing.T) {
	cache := &debounceListCache{memoryListCache: newMemoryListCache()}
	svc := NewService(db.New(newFakeDB(t)), nil, testCfg(), WithListCache(cache))
	wsID := uuid.NewString()
	key := listCacheKey(wsID)
	if err := cache.Set(context.Background(), key, []cachedRoomListItem{{ID: uuid.NewString(), Name: "A"}}, time.Minute); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	svc.SoftInvalidateListCache(context.Background(), wsID)
	if err := cache.Get(context.Background(), key, &[]cachedRoomListItem{}); err == nil {
		t.Fatal("expected cache cleared after first soft invalidate")
	}
	if err := cache.Set(context.Background(), key, []cachedRoomListItem{{ID: uuid.NewString(), Name: "B"}}, time.Minute); err != nil {
		t.Fatalf("reseed cache: %v", err)
	}
	svc.SoftInvalidateListCache(context.Background(), wsID)
	var still []cachedRoomListItem
	if err := cache.Get(context.Background(), key, &still); err != nil {
		t.Fatalf("debounced soft invalidate should keep cache: %v", err)
	}
	if len(still) != 1 || still[0].Name != "B" {
		t.Fatalf("unexpected cached payload after debounce: %+v", still)
	}
}

func TestListRoomsPageFiltersByQuery(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Query Workspace",
		Slug:     "query-workspace",
	}
	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "alpha-room",
		Name: "Alpha Financing",
	}); err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "beta-room",
		Name: "Beta Ops",
	}); err != nil {
		t.Fatalf("create beta: %v", err)
	}

	page, err := svc.ListRoomsPage(context.Background(), wsID, 1, 24, "financing")
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].Room.Name != "Alpha Financing" {
		t.Fatalf("unexpected items: %+v", page.Items)
	}
}

func TestListRoomsPageTreatsPercentAndUnderscoreLiterally(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Query Workspace",
		Slug:     "query-workspace",
	}
	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "percent-room",
		Name: "100% Club",
	}); err != nil {
		t.Fatalf("create percent: %v", err)
	}
	if _, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "ops-room",
		Name: "Beta Ops",
	}); err != nil {
		t.Fatalf("create ops: %v", err)
	}

	page, err := svc.ListRoomsPage(context.Background(), wsID, 1, 24, "%")
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Room.Name != "100% Club" {
		t.Fatalf("percent search = %+v", page)
	}

	wide, err := svc.ListRoomsPage(context.Background(), wsID, 1, 24, "_")
	if err != nil {
		t.Fatalf("underscore search: %v", err)
	}
	if wide.Total != 0 {
		t.Fatalf("underscore search matched %d rooms, want 0", wide.Total)
	}
}

func TestCreateRoomRetriesDuplicateSlug(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Slug Workspace",
		Slug:     "slug-workspace",
	}
	first, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "shared-room",
		Name: "First",
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if first.Slug != "shared-room" {
		t.Fatalf("first slug = %q", first.Slug)
	}
	second, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "shared-room",
		Name: "Second",
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.Slug != "shared-room-2" {
		t.Fatalf("second slug = %q, want shared-room-2", second.Slug)
	}
}
