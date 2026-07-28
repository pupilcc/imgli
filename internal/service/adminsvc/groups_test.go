package adminsvc

import (
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

// newGroup 是测试夹具：构造一个校验可过的合法组（供各用例在此基础上改字段）。
func newGroup(name string) *model.UserGroup {
	return &model.UserGroup{
		Name: name, StorageQuota: 1 << 30, MaxFileSize: 10 << 20,
		RatePerMinute: 10, RatePerHour: 100, RatePerDay: 500,
		AllowedExts: []string{"png", "jpg"},
	}
}

func TestCreateGroupValidationMatrix(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	if err := svc.CreateGroup(&model.UserGroup{Name: "", StorageQuota: 1, MaxFileSize: 1, AllowedExts: []string{"png"}}); err != ErrGroupNameInvalid {
		t.Errorf("空名 err = %v, want ErrGroupNameInvalid", err)
	}
	tooLong := make([]byte, 65)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if err := svc.CreateGroup(&model.UserGroup{Name: string(tooLong), StorageQuota: 1, MaxFileSize: 1, AllowedExts: []string{"png"}}); err != ErrGroupNameInvalid {
		t.Errorf("超长名 err = %v, want ErrGroupNameInvalid", err)
	}

	g := newGroup("q")
	g.StorageQuota = 0
	if err := svc.CreateGroup(g); err != ErrQuotaInvalid {
		t.Errorf("quota=0 err = %v, want ErrQuotaInvalid", err)
	}
	g = newGroup("q2")
	g.MaxFileSize = -1
	if err := svc.CreateGroup(g); err != ErrQuotaInvalid {
		t.Errorf("maxFileSize<0 err = %v, want ErrQuotaInvalid", err)
	}

	g = newGroup("e")
	g.AllowedExts = nil
	if err := svc.CreateGroup(g); err != ErrExtsEmpty {
		t.Errorf("exts=nil err = %v, want ErrExtsEmpty", err)
	}
	g = newGroup("e2")
	g.AllowedExts = []string{}
	if err := svc.CreateGroup(g); err != ErrExtsEmpty {
		t.Errorf("exts=[] err = %v, want ErrExtsEmpty", err)
	}
	g = newGroup("e3")
	g.AllowedExts = []string{"png", "  "}
	if err := svc.CreateGroup(g); err != ErrExtsEmpty {
		t.Errorf("exts 含空白项 err = %v, want ErrExtsEmpty", err)
	}

	g = newGroup("p")
	g.AllowedPolicyIDs = []uint64{999999}
	if err := svc.CreateGroup(g); err != ErrPolicyNotFound {
		t.Errorf("不存在策略 err = %v, want ErrPolicyNotFound", err)
	}
}

func TestCreateGroupNormalizesExtsAndPersists(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	g := newGroup("vip")
	g.AllowedExts = []string{"PNG", " Jpg ", "GIF", "png"} // 末尾 "png" 与首项大小写不同的重复项，验证去重
	g.AllowedPolicyIDs = []uint64{1}                       // 本地策略 seed id=1
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	if g.ID == 0 {
		t.Fatal("创建后 ID 应非 0")
	}
	want := []string{"png", "jpg", "gif"}
	if len(g.AllowedExts) != len(want) {
		t.Fatalf("AllowedExts = %v, want %v（应去重，[\"PNG\",\"png\"] → [\"png\"]）", g.AllowedExts, want)
	}
	for i, e := range want {
		if g.AllowedExts[i] != e {
			t.Errorf("AllowedExts[%d] = %q, want %q（应小写化）", i, g.AllowedExts[i], e)
		}
	}
	if g.IsDefault || g.IsGuest {
		t.Errorf("API 创建的组不应为内置组: IsDefault=%v IsGuest=%v", g.IsDefault, g.IsGuest)
	}

	var got model.UserGroup
	if err := db.First(&got, g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(got.AllowedExts) != 3 || got.AllowedExts[0] != "png" {
		t.Errorf("db 中 AllowedExts = %v", got.AllowedExts)
	}
}

func TestCreateGroupEmptyPolicyIDsAllowed(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("free")
	g.AllowedPolicyIDs = nil
	if err := svc.CreateGroup(g); err != nil {
		t.Errorf("空 AllowedPolicyIDs 应允许，err = %v", err)
	}
}

func TestListGroupsOrderAndUserCount(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	extra := newGroup("extra")
	if err := svc.CreateGroup(extra); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "a", Email: "a@x", GroupID: extra.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "b", Email: "b@x", GroupID: extra.ID}).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := svc.ListGroups()
	if err != nil {
		t.Fatal(err)
	}
	// TestDB 播种了默认组+游客组，此处应再加 extra，共 3 个，按 id 升序
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].Group.ID <= rows[i-1].Group.ID {
			t.Errorf("未按 id 升序: rows[%d].ID=%d <= rows[%d].ID=%d", i, rows[i].Group.ID, i-1, rows[i-1].Group.ID)
		}
	}
	var extraRow *GroupRow
	for i := range rows {
		if rows[i].Group.ID == extra.ID {
			extraRow = &rows[i]
		}
	}
	if extraRow == nil {
		t.Fatal("列表中未找到新建组")
	}
	if extraRow.UserCount != 2 {
		t.Errorf("extra.UserCount = %d, want 2", extraRow.UserCount)
	}
	// 默认/游客组此时应无用户，UserCount=0
	for i := range rows {
		if rows[i].Group.ID != extra.ID && rows[i].UserCount != 0 {
			t.Errorf("组 %d UserCount = %d, want 0", rows[i].Group.ID, rows[i].UserCount)
		}
	}
}

func strPtr(s string) *string       { return &s }
func i64Ptr(n int64) *int64         { return &n }
func intPtr(n int) *int             { return &n }
func extsPtr(e []string) *[]string  { return &e }
func idsPtr(ids []uint64) *[]uint64 { return &ids }

func TestUpdateGroupBuiltinNameRejectedQuotaAllowed(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	var def model.UserGroup
	if err := db.Where("is_default = ?", true).First(&def).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.UpdateGroup(def.ID, GroupPatch{Name: strPtr("改名")}); err != ErrBuiltinGroup {
		t.Errorf("内置组改名 err = %v, want ErrBuiltinGroup", err)
	}

	got, err := svc.UpdateGroup(def.ID, GroupPatch{StorageQuota: i64Ptr(999), RatePerMinute: intPtr(7)})
	if err != nil {
		t.Fatalf("内置组改配额/限速应允许: %v", err)
	}
	if got.StorageQuota != 999 || got.RatePerMinute != 7 {
		t.Errorf("got = %+v", got)
	}
	if got.Name != def.Name {
		t.Errorf("Name 应保持不变 = %s, want %s", got.Name, def.Name)
	}

	var guest model.UserGroup
	if err := db.Where("is_guest = ?", true).First(&guest).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateGroup(guest.ID, GroupPatch{Name: strPtr("x")}); err != ErrBuiltinGroup {
		t.Errorf("游客组改名 err = %v, want ErrBuiltinGroup", err)
	}
}

func TestUpdateGroupValidationMatrix(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("target")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.UpdateGroup(g.ID, GroupPatch{Name: strPtr("")}); err != ErrGroupNameInvalid {
		t.Errorf("空名 err = %v, want ErrGroupNameInvalid", err)
	}
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{StorageQuota: i64Ptr(0)}); err != ErrQuotaInvalid {
		t.Errorf("quota=0 err = %v, want ErrQuotaInvalid", err)
	}
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{MaxFileSize: i64Ptr(-1)}); err != ErrQuotaInvalid {
		t.Errorf("maxFileSize<0 err = %v, want ErrQuotaInvalid", err)
	}
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{AllowedExts: extsPtr(nil)}); err != ErrExtsEmpty {
		t.Errorf("exts=nil err = %v, want ErrExtsEmpty", err)
	}
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{AllowedExts: extsPtr([]string{})}); err != ErrExtsEmpty {
		t.Errorf("exts=[] err = %v, want ErrExtsEmpty", err)
	}
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{AllowedPolicyIDs: idsPtr([]uint64{999999})}); err != ErrPolicyNotFound {
		t.Errorf("不存在策略 err = %v, want ErrPolicyNotFound", err)
	}
	if _, err := svc.UpdateGroup(999999, GroupPatch{Name: strPtr("x")}); err != ErrGroupNotFound {
		t.Errorf("不存在组 err = %v, want ErrGroupNotFound", err)
	}
}

func TestUpdateGroupNormalizesAndPersists(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("target2")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}

	got, err := svc.UpdateGroup(g.ID, GroupPatch{
		Name:             strPtr("renamed"),
		StorageQuota:     i64Ptr(2 << 30),
		AllowedExts:      extsPtr([]string{"WEBP", "Avif"}),
		AllowedPolicyIDs: idsPtr([]uint64{1}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "renamed" || got.StorageQuota != 2<<30 {
		t.Errorf("got = %+v", got)
	}
	if len(got.AllowedExts) != 2 || got.AllowedExts[0] != "webp" || got.AllowedExts[1] != "avif" {
		t.Errorf("AllowedExts = %v, want [webp avif]", got.AllowedExts)
	}
	if len(got.AllowedPolicyIDs) != 1 || got.AllowedPolicyIDs[0] != 1 {
		t.Errorf("AllowedPolicyIDs = %v, want [1]", got.AllowedPolicyIDs)
	}

	var reload model.UserGroup
	if err := db.First(&reload, g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Name != "renamed" || len(reload.AllowedExts) != 2 {
		t.Errorf("db reload = %+v", reload)
	}

	// 未改字段（RatePerMinute 等）应保持原值不被清零
	if reload.RatePerMinute != g.RatePerMinute {
		t.Errorf("未patch字段被改动: RatePerMinute = %d, want %d", reload.RatePerMinute, g.RatePerMinute)
	}
}

func TestUpdateGroupEmptyPolicyIDsAllowed(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("target3")
	g.AllowedPolicyIDs = []uint64{1}
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	got, err := svc.UpdateGroup(g.ID, GroupPatch{AllowedPolicyIDs: idsPtr([]uint64{})})
	if err != nil {
		t.Fatalf("清空 AllowedPolicyIDs 应允许: %v", err)
	}
	if len(got.AllowedPolicyIDs) != 0 {
		t.Errorf("AllowedPolicyIDs = %v, want 空", got.AllowedPolicyIDs)
	}
}

// TestUpdateGroupPartialSelectAvoidsLostUpdate 顺序化模拟并发场景：First 取到 stale
// 快照后（此时 quota 还是旧值），"另一连接"先把 quota 改掉并提交成功；随后基于那份 stale
// 快照只对 rate_per_minute 这一列写回（重放 UpdateGroup 内部 Select(cols).Updates(&g)
// 所用的同一条语句——Model(&stale).Select(cols).Updates(&stale)）。若写回是整行覆盖
// （旧版 Save 写法），quota 会被 stale 里的旧值覆盖回去（lost update）；只 Select 实际
// patch 到的列则不会波及未涉及的 quota 列。
func TestUpdateGroupPartialSelectAvoidsLostUpdate(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("race")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}

	// 模拟 UpdateGroup 内部 First 阶段读到的快照（quota 仍是创建时的旧值）。
	var stale model.UserGroup
	if err := db.First(&stale, g.ID).Error; err != nil {
		t.Fatal(err)
	}

	// "另一连接"并发把 quota 改掉并提交成功。
	newQuota := int64(5 << 30)
	if _, err := svc.UpdateGroup(g.ID, GroupPatch{StorageQuota: &newQuota}); err != nil {
		t.Fatal(err)
	}

	// 基于陈旧快照，只对 rate_per_minute 这一列写回（重放生产代码写步所用的同一条语句）。
	stale.RatePerMinute = 42
	res := db.Model(&stale).Select([]string{"rate_per_minute"}).Updates(&stale)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("RowsAffected = %d, want 1", res.RowsAffected)
	}

	var reload model.UserGroup
	if err := db.First(&reload, g.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.StorageQuota != newQuota {
		t.Errorf("quota 被陈旧快照覆盖回旧值 = %d, want %d（lost update）", reload.StorageQuota, newQuota)
	}
	if reload.RatePerMinute != 42 {
		t.Errorf("rate_per_minute = %d, want 42", reload.RatePerMinute)
	}
}

func TestDeleteGroupBuiltinRejected(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	var def, guest model.UserGroup
	db.Where("is_default = ?", true).First(&def)
	db.Where("is_guest = ?", true).First(&guest)

	if err := svc.DeleteGroup(def.ID); err != ErrBuiltinGroup {
		t.Errorf("删默认组 err = %v, want ErrBuiltinGroup", err)
	}
	if err := svc.DeleteGroup(guest.ID); err != ErrBuiltinGroup {
		t.Errorf("删游客组 err = %v, want ErrBuiltinGroup", err)
	}
}

func TestDeleteGroupInUseRejected(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("busy")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "u", Email: "u@x", GroupID: g.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteGroup(g.ID); err != ErrGroupInUse {
		t.Errorf("删有用户的组 err = %v, want ErrGroupInUse", err)
	}
}

func TestDeleteGroupNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if err := svc.DeleteGroup(999999); err != ErrGroupNotFound {
		t.Errorf("err = %v, want ErrGroupNotFound", err)
	}
}

func TestDeleteGroupSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("empty")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.UserGroup{}).Where("id = ?", g.ID).Count(&n)
	if n != 0 {
		t.Errorf("删除后仍存在，count = %d", n)
	}
}

// TestDeleteGroupReplayNotFound 对已删 id 重放 DeleteGroup，断言 ErrGroupNotFound（幂等，不假成功）。
func TestDeleteGroupReplayNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("onceonly")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteGroup(g.ID); err != ErrGroupNotFound {
		t.Errorf("重复删除 err = %v, want ErrGroupNotFound", err)
	}
}

// TestDeleteGroupRowsAffectedGateNoFakeSuccess 直接验证 DeleteGroup 末尾所用的同一条删除语句：
// 对已不存在的行必须 0 行受影响（同 T3 AdminSoftDelete 的既有门禁测试风格），
// 确认该分支不是死代码——正常路径下 First 已能拦掉大多数"已不存在"场景，
// 但末尾这道门禁专为 First 与 Delete 之间的并发删除窗口兜底。
func TestDeleteGroupRowsAffectedGateNoFakeSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	g := newGroup("gatekey")
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	// 重放 DeleteGroup 内部所用的同一条语句：对已不存在的行必须 0 行受影响。
	res := db.Delete(&model.UserGroup{}, "id = ?", g.ID)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("门禁失效：对已删行 Delete 应 0 行受影响, got %d", res.RowsAffected)
	}
}
