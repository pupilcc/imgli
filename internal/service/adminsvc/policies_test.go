package adminsvc

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yixian-huang/imgli/internal/model"
)

func strP(s string) *string { return &s }
func boolP(b bool) *bool    { return &b }

// newLocalPolicyBody 是通过校验的最小合法 local 策略（root 用调用方传入的目录）。
func newLocalPolicy(name, root string) *model.StoragePolicy {
	return &model.StoragePolicy{
		Name: name, Driver: "local",
		Config: map[string]string{"root": root},
	}
}

func TestCreatePolicyValidationMatrix(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "", Driver: "local", Config: map[string]string{"root": "x"}}); err != ErrPolicyNameInvalid {
		t.Errorf("空名 err = %v, want ErrPolicyNameInvalid", err)
	}
	tooLong := make([]byte, 65)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if err := svc.CreatePolicy(&model.StoragePolicy{Name: string(tooLong), Driver: "local", Config: map[string]string{"root": "x"}}); err != ErrPolicyNameInvalid {
		t.Errorf("超长名 err = %v, want ErrPolicyNameInvalid", err)
	}

	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "s3one", Driver: "s3", Config: map[string]string{"root": "x"}}); err != ErrBadConfig {
		t.Errorf("s3 缺必填字段 err = %v, want ErrBadConfig", err)
	}
	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "webdavone", Driver: "webdav", Config: map[string]string{"root": "x"}}); err != ErrBadConfig {
		t.Errorf("webdav 缺 endpoint err = %v, want ErrBadConfig", err)
	}
	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "ossone", Driver: "oss", Config: map[string]string{"root": "x"}}); err != ErrDriverUnsupported {
		t.Errorf("非 local|s3|webdav driver err = %v, want ErrDriverUnsupported", err)
	}

	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "noroot", Driver: "local", Config: map[string]string{}}); err != ErrBadConfig {
		t.Errorf("config 无 root err = %v, want ErrBadConfig", err)
	}
	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "emptyroot", Driver: "local", Config: map[string]string{"root": "  "}}); err != ErrBadConfig {
		t.Errorf("config root 空白 err = %v, want ErrBadConfig", err)
	}
	if err := svc.CreatePolicy(&model.StoragePolicy{Name: "nilconfig", Driver: "local", Config: nil}); err != ErrBadConfig {
		t.Errorf("config nil err = %v, want ErrBadConfig", err)
	}
}

func TestCreatePolicyPersistsWithDefaults(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newLocalPolicy("我的存储", "/data/x")
	// 显式设 true——本测试只关心 PathTemplate 的默认填充，Enabled 的语义（未设置 vs
	// 显式 false）由 TestCreatePolicyRespectsExplicitEnabledFalse/
	// TestCreatePolicyUnsetEnabledPersistsFalse 单独覆盖，此处避免混淆两个关注点。
	p.Enabled = true
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 {
		t.Fatal("创建后 ID 应非 0")
	}
	if p.PathTemplate != "{Y}/{m}/{d}/{uniqid}.{ext}" {
		t.Errorf("PathTemplate 空时应用②b默认, got %q", p.PathTemplate)
	}
	if !p.Enabled {
		t.Errorf("Enabled 显式传 true 应保持 true")
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.PathTemplate != p.PathTemplate || reload.Config["root"] != "/data/x" {
		t.Errorf("reload = %+v", reload)
	}
}

func TestCreatePolicyCustomPathTemplatePersists(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newLocalPolicy("custom", "/data/y")
	p.PathTemplate = "{Y}-{m}/{uniqid}.{ext}"
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.PathTemplate != "{Y}-{m}/{uniqid}.{ext}" {
		t.Errorf("PathTemplate 应保留自定义值, got %q", p.PathTemplate)
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.PathTemplate != p.PathTemplate {
		t.Errorf("reload.PathTemplate = %q, want %q", reload.PathTemplate, p.PathTemplate)
	}
}

// TestCreatePolicyRespectsExplicitEnabledFalse 锁定修复后的行为：Enabled 字段带
// `gorm:"default:true"`，GORM Create() 对 bool 零值(false)+字面量默认值的字段会自动回填
// 默认值 true（连同改写调用方传入的结构体字段本身），CreatePolicy 内部在 Create 前记录
// 调用方意图，Create 后若被静默改回 true 则用一条单列 UPDATE 纠正——端到端断言 p.Enabled
// 与 DB 中持久化的值均须是调用方显式传入的 false，而不是被 GORM 默认值悄悄顶替。
func TestCreatePolicyRespectsExplicitEnabledFalse(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newLocalPolicy("explicit-false", "/data/z")
	p.Enabled = false
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.Enabled {
		t.Errorf("返回对象 Enabled = true, want false（调用方显式要求禁用）")
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Enabled {
		t.Errorf("DB 中 Enabled = true, want false——GORM default 覆盖未被纠正")
	}
}

// TestCreatePolicyUnsetEnabledPersistsFalse 记录服务层的实际语义：CreatePolicy 不对
// Enabled 做隐式默认——bool 零值(false)在类型层面无法区分"调用方未设置"与"调用方显式要
// false"，服务层选择忠实持久化调用方给的值（含零值）。"POST 请求未带 enabled 字段时新建
// 应默认启用"是 HTTP 层的产品决策，由 handler.CreatePolicy 在调用本方法前把默认值 true
// 显式填入 p.Enabled 来实现（req.Enabled == nil 时 p.Enabled = true，见
// internal/handler/admin.go 与 TestAdminPoliciesCRUDFlow 的契约覆盖），不是本方法的职责。
// 此测试防止未来有人误以为服务层"漏了默认值"而引入与 wantDisabled 逻辑冲突的改动。
func TestCreatePolicyUnsetEnabledPersistsFalse(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newLocalPolicy("unset-enabled", "/data/unset")
	// p.Enabled 保持 Go 零值 false，模拟绕过 handler 直接调用服务层且未触碰该字段。
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.Enabled {
		t.Errorf("Enabled = true, want false（服务层不做隐式默认，见函数注释）")
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Enabled {
		t.Errorf("DB 中 Enabled = true, want false")
	}
}

func TestListPoliciesOrderAndAggregates(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p2 := newLocalPolicy("second", "/data/2")
	if err := svc.CreatePolicy(p2); err != nil {
		t.Fatal(err)
	}
	// id=1 是 TestDB 播种的本地策略；给它挂两个文件 + live/trash 图
	fa := &model.File{Hash: "fa", StoragePolicyID: 1, Path: "a", Size: 100, RefCount: 1}
	fb := &model.File{Hash: "fb", StoragePolicyID: 1, Path: "b", Size: 250, RefCount: 1}
	if err := db.Create(fa).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(fb).Error; err != nil {
		t.Fatal(err)
	}
	liveImg := &model.Image{Key: "livekey00001", FileID: fa.ID, Name: "live.png", Ext: "png", Visibility: "public", Status: "normal"}
	trashImg := &model.Image{Key: "trashkey0001", FileID: fb.ID, Name: "trash.png", Ext: "png", Visibility: "public", Status: "normal"}
	if err := db.Create(liveImg).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(trashImg).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(trashImg).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := svc.ListPolicies()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].Policy.ID <= rows[i-1].Policy.ID {
			t.Errorf("未按 id 升序: rows[%d].ID=%d <= rows[%d].ID=%d", i, rows[i].Policy.ID, i-1, rows[i-1].Policy.ID)
		}
	}
	var seeded, second *PolicyRow
	for i := range rows {
		switch rows[i].Policy.ID {
		case 1:
			seeded = &rows[i]
		case p2.ID:
			second = &rows[i]
		}
	}
	if seeded == nil || second == nil {
		t.Fatalf("行未找全: %+v", rows)
	}
	if seeded.FileCount != 2 || seeded.UsedBytes != 350 {
		t.Errorf("seeded files = %+v, want FileCount=2 UsedBytes=350", *seeded)
	}
	if seeded.LiveImageCount != 1 || seeded.TrashImageCount != 1 {
		t.Errorf("seeded images live=%d trash=%d, want 1/1", seeded.LiveImageCount, seeded.TrashImageCount)
	}
	if second.FileCount != 0 || second.UsedBytes != 0 || second.LiveImageCount != 0 || second.TrashImageCount != 0 {
		t.Errorf("second = %+v, want zeros", *second)
	}
}

func TestUpdatePolicyPartialFields(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("orig", "/data/orig")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	got, err := svc.UpdatePolicy(p.ID, PolicyPatch{
		Name:      strP("renamed"),
		CDNDomain: strP("https://cdn.example.com"),
		Enabled:   boolP(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "renamed" || got.CDNDomain != "https://cdn.example.com" || got.Enabled {
		t.Errorf("got = %+v", got)
	}
	if got.Config["root"] != "/data/orig" {
		t.Errorf("未 patch 的 config 应保持不变, got %+v", got.Config)
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Name != "renamed" || reload.Enabled {
		t.Errorf("reload = %+v", reload)
	}
}

func TestUpdatePolicyConfig(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("cfgtest", "/data/old")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	got, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(`{"root":"/data/new"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config["root"] != "/data/new" {
		t.Errorf("Config[root] = %q, want /data/new", got.Config["root"])
	}
}

func TestUpdatePolicyBadConfigRejected(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("cfgbad", "/data/old")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(`not json`)}); err != ErrBadConfig {
		t.Errorf("非法 JSON err = %v, want ErrBadConfig", err)
	}
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(`{}`)}); err != ErrBadConfig {
		t.Errorf("缺 root err = %v, want ErrBadConfig", err)
	}
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(`{"root":""}`)}); err != ErrBadConfig {
		t.Errorf("root 空 err = %v, want ErrBadConfig", err)
	}

	// 校验失败不应改动原值
	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Config["root"] != "/data/old" {
		t.Errorf("校验失败后 config 被改动: %+v", reload.Config)
	}
}

func TestUpdatePolicyNameInvalid(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("nametest", "/data/n")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Name: strP("")}); err != ErrPolicyNameInvalid {
		t.Errorf("空名 err = %v, want ErrPolicyNameInvalid", err)
	}
}

func TestUpdatePolicyNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if _, err := svc.UpdatePolicy(999999, PolicyPatch{Name: strP("x")}); err != ErrPolicyNotFound {
		t.Errorf("err = %v, want ErrPolicyNotFound", err)
	}
}

// TestUpdatePolicyPartialSelectAvoidsLostUpdate 与 groups.go 同款铁律验证：
// 只 Select 实际 patch 到的列写回，未涉及字段（如并发改动的 config）不应被陈旧快照覆盖。
func TestUpdatePolicyPartialSelectAvoidsLostUpdate(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("race", "/data/race")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	var stale model.StoragePolicy
	if err := db.First(&stale, p.ID).Error; err != nil {
		t.Fatal(err)
	}

	// "另一连接"并发把 config 改掉并提交成功。
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(`{"root":"/data/concurrent"}`)}); err != nil {
		t.Fatal(err)
	}

	// 基于陈旧快照，只对 cdn_domain 这一列写回（重放生产代码写步所用的同一条语句）。
	stale.CDNDomain = "stale.example.com"
	res := db.Model(&stale).Select([]string{"cdn_domain"}).Updates(&stale)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("RowsAffected = %d, want 1", res.RowsAffected)
	}

	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Config["root"] != "/data/concurrent" {
		t.Errorf("config 被陈旧快照覆盖回旧值 = %+v, want root=/data/concurrent（lost update）", reload.Config)
	}
	if reload.CDNDomain != "stale.example.com" {
		t.Errorf("cdn_domain = %s, want stale.example.com", reload.CDNDomain)
	}
}

func TestDeletePolicyInUseRejected(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	// id=1 是 TestDB 播种的本地策略
	if err := db.Create(&model.File{Hash: "busyf", StoragePolicyID: 1, Path: "p", Size: 1, RefCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePolicy(1); err != ErrPolicyInUse {
		t.Errorf("err = %v, want ErrPolicyInUse", err)
	}
}

func TestDeletePolicyNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if err := svc.DeletePolicy(999999); err != ErrPolicyNotFound {
		t.Errorf("err = %v, want ErrPolicyNotFound", err)
	}
}

func TestDeletePolicySuccessAndReplayNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("disposable", "/data/d")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePolicy(p.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.StoragePolicy{}).Where("id = ?", p.ID).Count(&n)
	if n != 0 {
		t.Errorf("删除后仍存在，count = %d", n)
	}
	// 重放：对已删 id 再删一次，须幂等地报 ErrPolicyNotFound，不假成功。
	if err := svc.DeletePolicy(p.ID); err != ErrPolicyNotFound {
		t.Errorf("重复删除 err = %v, want ErrPolicyNotFound", err)
	}
}

// TestDeletePolicyRowsAffectedGateNoFakeSuccess 直接验证 DeletePolicy 末尾所用的同一条删除语句：
// 对已不存在的行必须 0 行受影响（T3/T4 铁律：门禁不是死代码）。
func TestDeletePolicyRowsAffectedGateNoFakeSuccess(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("gatekey", "/data/g")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePolicy(p.ID); err != nil {
		t.Fatal(err)
	}
	res := db.Delete(&model.StoragePolicy{}, "id = ?", p.ID)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("门禁失效：对已删行 Delete 应 0 行受影响, got %d", res.RowsAffected)
	}
}

func TestTestPolicySuccessRealProbe(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	root := t.TempDir()
	p := newLocalPolicy("probe-ok", root)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	latency, err := svc.TestPolicy(p.ID)
	if err != nil {
		t.Fatalf("TestPolicy 失败: %v", err)
	}
	if latency < 0 {
		t.Errorf("latencyMs = %d, want >= 0", latency)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("探针文件未清理干净，root 中残留: %s", e.Name())
	}
}

func TestTestPolicyBadRootFails(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	base := t.TempDir()
	// 用一个已存在的普通文件占住路径，令其不能被当作目录使用（MkdirAll 必失败）。
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badRoot := filepath.Join(blocker, "sub")
	p := newLocalPolicy("probe-bad", badRoot)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	_, err := svc.TestPolicy(p.ID)
	if err == nil {
		t.Fatal("坏 root 应返回描述性 error，got nil")
	}
	s := err.Error()
	if !strings.Contains(s, "root 不可写") || !strings.Contains(s, badRoot) {
		t.Errorf("应含动作与路径: %v", err)
	}
}

// TestTestPolicyRelativeRootJoinsDataDir 覆盖默认种子 root=uploads 场景：
// 探针必须写到 DataDir/uploads，而非进程 CWD 下的 ./uploads。
func TestTestPolicyRelativeRootJoinsDataDir(t *testing.T) {
	db := model.TestDB(t)
	dataDir := t.TempDir()
	svc := New(db).UseDataDir(dataDir)
	p := newLocalPolicy("rel-uploads", "uploads")
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestPolicy(p.ID); err != nil {
		t.Fatalf("相对 root 拼 DataDir 后应可写: %v", err)
	}
	// MkdirAll 应已创建 DataDir/uploads（探针文件会清理，目录可保留）
	want := filepath.Join(dataDir, "uploads")
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatalf("期望探针目录 %s 存在, err=%v", want, err)
	}
	// 不应在 CWD 下误建 uploads（若 CWD 恰好可写会误导；仅断言 DataDir 侧已建）
	entries, err := os.ReadDir(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".imgli-probe-") {
			t.Errorf("探针文件未清理: %s", e.Name())
		}
	}
}

// TestTestPolicyAbsoluteRootIgnoresDataDir 绝对 root 原样探针，不拼进 DataDir。
func TestTestPolicyAbsoluteRootIgnoresDataDir(t *testing.T) {
	db := model.TestDB(t)
	absRoot := t.TempDir()
	otherData := t.TempDir()
	svc := New(db).UseDataDir(otherData)
	p := newLocalPolicy("abs-root", absRoot)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestPolicy(p.ID); err != nil {
		t.Fatalf("绝对 root 探针应成功: %v", err)
	}
	if entries, err := os.ReadDir(otherData); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Errorf("绝对 root 不应在 DataDir 落文件, got %v", entries)
	}
}

func TestLocalProbeRootJoin(t *testing.T) {
	svc := New(model.TestDB(t)).UseDataDir("/var/imgli-data")
	if got := svc.localProbeRoot("uploads"); got != filepath.Join("/var/imgli-data", "uploads") {
		t.Errorf("relative = %q", got)
	}
	if got := svc.localProbeRoot("/abs/store"); got != "/abs/store" {
		t.Errorf("absolute = %q, want /abs/store", got)
	}
	if got := svc.localProbeRoot(""); got != filepath.Join("/var/imgli-data", "uploads") {
		t.Errorf("empty default = %q", got)
	}
	if got := svc.localProbeRoot("  "); got != filepath.Join("/var/imgli-data", "uploads") {
		t.Errorf("blank default = %q", got)
	}
}

// TestDeletePolicyRejectedWhenReferencedByGroup 覆盖 FK 管不到的场景：allowed_policy_ids
// 是 JSON 数组列，删除策略时约束不会拦——需要服务层自己查组引用（RESTRICT 语义，同 files 引用）。
func TestDeletePolicyRejectedWhenReferencedByGroup(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := &model.StoragePolicy{Name: "备用", Driver: "local", Config: map[string]string{"root": "u"}}
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	g := &model.UserGroup{Name: "组X", StorageQuota: 1 << 30, MaxFileSize: 1 << 20,
		AllowedExts: []string{"png"}, AllowedPolicyIDs: []uint64{p.ID}}
	if err := svc.CreateGroup(g); err != nil {
		t.Fatal(err)
	}
	// 该策略无 files 引用，但被组引用 → 删应被拒
	if err := svc.DeletePolicy(p.ID); !errors.Is(err, ErrPolicyInUseByGroup) {
		t.Fatalf("被组引用的策略删除应返回 ErrPolicyInUseByGroup, got %v", err)
	}
}

func TestTestPolicyNotFound(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	if _, err := svc.TestPolicy(999999); err != ErrPolicyNotFound {
		t.Errorf("err = %v, want ErrPolicyNotFound", err)
	}
}

func TestTestPolicyDriverUnsupported(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	// 直接绕过 CreatePolicy 的白名单校验造一个未实现驱动（模拟历史数据/未来驱动接入前的探测请求）。
	pol := &model.StoragePolicy{Name: "ossx", Driver: "oss", Config: map[string]string{"root": "x"}}
	if err := db.Create(pol).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestPolicy(pol.ID); err != ErrDriverUnsupported {
		t.Errorf("err = %v, want ErrDriverUnsupported", err)
	}
}

func validS3Config() map[string]string {
	return map[string]string{
		"endpoint":          "s3.example.com",
		"region":            "us-east-1",
		"bucket":            "mybucket",
		"access_key_id":     "AKIDEXAMPLE0000",
		"secret_access_key": "SECRETxxxx",
		"path_style":        "true",
	}
}

func newS3Policy(name string, cfg map[string]string) *model.StoragePolicy {
	return &model.StoragePolicy{Name: name, Driver: "s3", Config: cfg}
}

func TestValidateS3ConfigAndDriverConfig(t *testing.T) {
	if err := validateS3Config(validS3Config()); err != nil {
		t.Errorf("合法 s3 config err = %v", err)
	}
	for _, miss := range []string{"endpoint", "region", "bucket", "access_key_id", "secret_access_key"} {
		cfg := validS3Config()
		cfg[miss] = ""
		if err := validateS3Config(cfg); err != ErrBadConfig {
			t.Errorf("缺 %s err = %v, want ErrBadConfig", miss, err)
		}
	}
	cfg := validS3Config()
	cfg["path_style"] = "x"
	if err := validateS3Config(cfg); err != ErrBadConfig {
		t.Errorf("path_style x err = %v, want ErrBadConfig", err)
	}
	// prefix 自动补尾 /
	cfg = validS3Config()
	cfg["prefix"] = "upload"
	if err := validateS3Config(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["prefix"] != "upload/" {
		t.Errorf("prefix = %q, want upload/", cfg["prefix"])
	}
	if err := validateDriverConfig("oss", map[string]string{}); err != ErrDriverUnsupported {
		t.Errorf("oss err = %v, want ErrDriverUnsupported", err)
	}
	if err := validateDriverConfig("s3", validS3Config()); err != nil {
		t.Errorf("s3 driver config err = %v", err)
	}
	if err := validateDriverConfig("local", map[string]string{"root": "/x"}); err != nil {
		t.Errorf("local driver config err = %v", err)
	}
}

func TestValidateCDNDomainMessages(t *testing.T) {
	if err := validateCDNDomain(""); err != nil {
		t.Fatal(err)
	}
	if err := validateCDNDomain("https://cdn.example.com"); err != nil {
		t.Fatal(err)
	}
	err := validateCDNDomain("bucket.oss-cn-hangzhou.aliyuncs.com")
	if !errors.Is(err, ErrBadConfig) {
		t.Fatalf("bare host: %v", err)
	}
	if !strings.Contains(err.Error(), "https://") {
		t.Errorf("message should mention scheme: %v", err)
	}
	err = validateCDNDomain("https://user:pass@cdn.example")
	if !errors.Is(err, ErrBadConfig) {
		t.Fatalf("userinfo: %v", err)
	}
}

func TestNormalizePathTemplate(t *testing.T) {
	pt, err := normalizePathTemplate("")
	if err != nil || pt != defaultPathTemplate {
		t.Fatalf("empty → default: %q %v", pt, err)
	}
	pt, err = normalizePathTemplate("{uniqid}.{ext}")
	if err != nil || pt != "{uniqid}.{ext}" {
		t.Fatalf("flat: %q %v", pt, err)
	}
	_, err = normalizePathTemplate("{Y}/{m}/{d}.{ext}")
	if !errors.Is(err, ErrBadConfig) {
		t.Fatalf("no random: %v", err)
	}
}

func TestCreatePolicyRejectsBadPathTemplateAndCDN(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newLocalPolicy("badtpl", "/data/z")
	p.PathTemplate = "{Y}.{ext}"
	if err := svc.CreatePolicy(p); !errors.Is(err, ErrBadConfig) {
		t.Fatalf("path template: %v", err)
	}
	p2 := newLocalPolicy("badcdn", "/data/z2")
	p2.CDNDomain = "cdn.example.com"
	if err := svc.CreatePolicy(p2); !errors.Is(err, ErrBadConfig) {
		t.Fatalf("cdn: %v", err)
	}
}

func TestCreatePolicyS3(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newS3Policy("s3-ok", validS3Config())
	p.Enabled = true
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatalf("CreatePolicy s3: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("创建后 ID 应非 0")
	}
	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Driver != "s3" || reload.Config["secret_access_key"] != "SECRETxxxx" {
		t.Errorf("reload = %+v", reload)
	}

	bad := newS3Policy("s3-nosecret", validS3Config())
	delete(bad.Config, "secret_access_key")
	if err := svc.CreatePolicy(bad); err != ErrBadConfig {
		t.Errorf("缺 secret err = %v, want ErrBadConfig", err)
	}
}

func TestUpdatePolicyS3SecretRetain(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newS3Policy("s3-retain", validS3Config())
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	// secret 打码回传且 endpoint/region/bucket/AKID 不变 → 沿用库中明文
	patch := `{"endpoint":"s3.example.com","region":"us-east-1","bucket":"mybucket","access_key_id":"AKIDEXAMPLE0000","secret_access_key":"****xxxx","path_style":"true"}`
	got, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(patch)})
	if err != nil {
		t.Fatalf("retain UpdatePolicy: %v", err)
	}
	if got.Config["secret_access_key"] != "SECRETxxxx" {
		t.Errorf("secret 应保留明文, got %q", got.Config["secret_access_key"])
	}

	// 改 endpoint + 掩码 secret → ErrBadConfig
	steal := `{"endpoint":"evil.example.com","region":"us-east-1","bucket":"mybucket","access_key_id":"AKIDEXAMPLE0000","secret_access_key":"****xxxx","path_style":"true"}`
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(steal)}); err != ErrBadConfig {
		t.Errorf("改 endpoint+掩码 secret err = %v, want ErrBadConfig", err)
	}

	// 改 bucket + 掩码 → ErrBadConfig
	stealB := `{"endpoint":"s3.example.com","region":"us-east-1","bucket":"other","access_key_id":"AKIDEXAMPLE0000","secret_access_key":"****xxxx","path_style":"true"}`
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(stealB)}); err != ErrBadConfig {
		t.Errorf("改 bucket+掩码 secret err = %v, want ErrBadConfig", err)
	}

	// 库中明文未变
	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Config["secret_access_key"] != "SECRETxxxx" || reload.Config["endpoint"] != "s3.example.com" {
		t.Errorf("失败后 config 被改: %+v", reload.Config)
	}
}

func TestTestPolicyS3Unreachable(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	cfg := validS3Config()
	cfg["endpoint"] = "127.0.0.1:1" // 拒连
	p := newS3Policy("s3-down", cfg)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	_, err := svc.TestPolicy(p.ID)
	if err == nil {
		t.Fatal("不可达 endpoint 应返回 error")
	}
	s := err.Error()
	if !strings.Contains(s, "探针") && !strings.Contains(s, "驱动") {
		t.Errorf("err 应说明失败: %v", err)
	}
	if !strings.Contains(s, "127.0.0.1:1") {
		t.Errorf("应含 endpoint: %v", err)
	}
}

func validWebDAVConfig() map[string]string {
	return map[string]string{
		"endpoint": "https://dav.example.com/imgli",
		"username": "alice",
		"password": "s3cretpass",
	}
}

func newWebDAVPolicy(name string, cfg map[string]string) *model.StoragePolicy {
	return &model.StoragePolicy{Name: name, Driver: "webdav", Config: cfg}
}

func TestValidateWebDAVConfig(t *testing.T) {
	if err := validateWebDAVConfig(validWebDAVConfig()); err != nil {
		t.Errorf("合法 webdav config err = %v", err)
	}
	// username/password 空仍合法
	if err := validateWebDAVConfig(map[string]string{"endpoint": "https://dav.example.com/imgli"}); err != nil {
		t.Errorf("无用户名密码 err = %v", err)
	}
	if err := validateWebDAVConfig(map[string]string{"endpoint": ""}); err != ErrBadConfig {
		t.Errorf("endpoint 空 err = %v, want ErrBadConfig", err)
	}
	if err := validateWebDAVConfig(map[string]string{}); err != ErrBadConfig {
		t.Errorf("无 endpoint err = %v, want ErrBadConfig", err)
	}
	for _, ep := range []string{"ftp://x", "not-a-url", "http://", "://bad"} {
		if err := validateWebDAVConfig(map[string]string{"endpoint": ep}); err != ErrBadConfig {
			t.Errorf("endpoint %q err = %v, want ErrBadConfig", ep, err)
		}
	}
	if err := validateDriverConfig("webdav", validWebDAVConfig()); err != nil {
		t.Errorf("webdav driver config err = %v", err)
	}
}

func TestCreatePolicyWebDAV(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)

	p := newWebDAVPolicy("webdav-ok", validWebDAVConfig())
	p.Enabled = true
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatalf("CreatePolicy webdav: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("创建后 ID 应非 0")
	}
	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Driver != "webdav" || reload.Config["password"] != "s3cretpass" {
		t.Errorf("reload = %+v", reload)
	}
	if reload.Config["endpoint"] != "https://dav.example.com/imgli" {
		t.Errorf("endpoint = %q", reload.Config["endpoint"])
	}

	bad := newWebDAVPolicy("webdav-noep", validWebDAVConfig())
	delete(bad.Config, "endpoint")
	if err := svc.CreatePolicy(bad); err != ErrBadConfig {
		t.Errorf("缺 endpoint err = %v, want ErrBadConfig", err)
	}
}

func TestUpdatePolicyWebDAVPasswordRetain(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	p := newWebDAVPolicy("webdav-retain", validWebDAVConfig())
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	// password 打码回传且 endpoint/username 不变 → 沿用库中明文
	patch := `{"endpoint":"https://dav.example.com/imgli","username":"alice","password":"****pass"}`
	got, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(patch)})
	if err != nil {
		t.Fatalf("retain UpdatePolicy: %v", err)
	}
	if got.Config["password"] != "s3cretpass" {
		t.Errorf("password 应保留明文, got %q", got.Config["password"])
	}

	// 改 endpoint + 掩码 password → ErrBadConfig
	steal := `{"endpoint":"https://evil.example.com/imgli","username":"alice","password":"****pass"}`
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(steal)}); err != ErrBadConfig {
		t.Errorf("改 endpoint+掩码 password err = %v, want ErrBadConfig", err)
	}

	// 改 username + 掩码 → ErrBadConfig
	stealU := `{"endpoint":"https://dav.example.com/imgli","username":"eve","password":"****pass"}`
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(stealU)}); err != ErrBadConfig {
		t.Errorf("改 username+掩码 password err = %v, want ErrBadConfig", err)
	}

	// 库中明文未变
	var reload model.StoragePolicy
	if err := db.First(&reload, p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reload.Config["password"] != "s3cretpass" || reload.Config["endpoint"] != "https://dav.example.com/imgli" {
		t.Errorf("失败后 config 被改: %+v", reload.Config)
	}
}

func TestTestPolicyWebDAVUnreachable(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	cfg := validWebDAVConfig()
	cfg["endpoint"] = "http://127.0.0.1:1" // 拒连
	p := newWebDAVPolicy("webdav-down", cfg)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	_, err := svc.TestPolicy(p.ID)
	if err == nil {
		t.Fatal("不可达 endpoint 应返回 error")
	}
	s := err.Error()
	if !strings.Contains(s, "webdav") || !strings.Contains(s, "http://127.0.0.1:1") {
		t.Errorf("应含 driver/endpoint: %v", err)
	}
}

// davProbeMock 最小 WebDAV：根下直接 PUT 返回 404；经 MKCOL 建父后再 PUT 成功。
// 用来证明 remoteProbePrefix（imgli-probe/…）能触发父集合创建，对齐真实上传路径形态。
type davProbeMock struct {
	mu      sync.Mutex
	objects map[string][]byte
	dirs    map[string]bool
	mkcols  []string
}

func (m *davProbeMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")
	key = strings.TrimSuffix(key, "/")
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = map[string][]byte{}
	}
	if m.dirs == nil {
		m.dirs = map[string]bool{}
	}
	switch r.Method {
	case http.MethodPut:
		if i := strings.LastIndex(key, "/"); i >= 0 {
			parent := key[:i]
			if !m.dirs[parent] {
				// 与「根 PUT 友好度差 / 缺父 404」的对端对齐
				http.NotFound(w, r)
				return
			}
		}
		body, _ := io.ReadAll(r.Body)
		m.objects[key] = body
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet, http.MethodHead:
		data, ok := m.objects[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", itoaLen(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	case http.MethodDelete:
		delete(m.objects, key)
		w.WriteHeader(http.StatusNoContent)
	case "MKCOL":
		m.mkcols = append(m.mkcols, key)
		m.dirs[key] = true
		w.WriteHeader(http.StatusCreated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func itoaLen(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestTestPolicyWebDAVSubdirProbeSucceeds(t *testing.T) {
	mock := &davProbeMock{}
	srv := httptest.NewServer(mock)
	t.Cleanup(srv.Close)

	db := model.TestDB(t)
	svc := New(db)
	p := newWebDAVPolicy("webdav-probe-ok", map[string]string{
		"endpoint": srv.URL,
		"username": "u",
		"password": "p",
	})
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestPolicy(p.ID); err != nil {
		t.Fatalf("带 imgli-probe/ 前缀的探针应成功: %v", err)
	}
	mock.mu.Lock()
	defer mock.mu.Unlock()
	found := false
	for _, c := range mock.mkcols {
		if c == strings.TrimSuffix(remoteProbePrefix, "/") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("应 MKCOL %q, got %v", strings.TrimSuffix(remoteProbePrefix, "/"), mock.mkcols)
	}
	if len(mock.objects) != 0 {
		t.Errorf("探针对象应已删除, residual=%v", mock.objects)
	}
}

func s3CfgWithPresign(domain string) map[string]string {
	return map[string]string{
		"endpoint": "s3.example.com", "region": "us-east-1", "bucket": "b",
		"access_key_id": "AK", "secret_access_key": "SK",
		"path_style": "true", "presign_domain": domain,
	}
}

func TestValidateS3PresignDomain(t *testing.T) {
	cases := []struct {
		name   string
		domain string
		ok     bool
	}{
		{"留空即不启用", "", true},
		{"https 合法", "https://s3.img.li", true},
		{"http 合法(内网/自建)", "http://10.0.0.5:9000", true},
		{"缺 scheme", "s3.img.li", false},
		{"非 http(s)", "ftp://s3.img.li", false},
		{"内联 userinfo", "https://u:p@s3.img.li", false},
		{"带 path", "https://s3.img.li/imgli", false},
		{"非 ASCII 主机名", "https://例子.com", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateS3Config(s3CfgWithPresign(c.domain))
			if c.ok && err != nil {
				t.Errorf("应通过,得到 %v", err)
			}
			if !c.ok && err == nil {
				t.Errorf("应拒绝 %q", c.domain)
			}
		})
	}
}

// TestUpdatePolicyPresignDomainRequiresSecret 改 presign_domain 时若 secret 仍是
// 掩码值必须拒绝——presign_domain 决定签名被签给哪个主机,属指向类字段,
// 与 endpoint/bucket 同等对待。
func TestUpdatePolicyPresignDomainRequiresSecret(t *testing.T) {
	db := model.TestDB(t)
	svc := New(db)
	cfg := validS3Config()
	cfg["presign_domain"] = "https://s3.img.li"
	p := newS3Policy("s3-presign", cfg)
	if err := svc.CreatePolicy(p); err != nil {
		t.Fatal(err)
	}

	// 只改 presign_domain + 掩码 secret → ErrBadConfig
	steal := `{"endpoint":"s3.example.com","region":"us-east-1","bucket":"mybucket","access_key_id":"AKIDEXAMPLE0000","secret_access_key":"****xxxx","path_style":"true","presign_domain":"https://evil.example"}`
	if _, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(steal)}); err != ErrBadConfig {
		t.Errorf("改 presign_domain+掩码 secret err = %v, want ErrBadConfig", err)
	}

	// presign_domain 不变 + 掩码 secret → 沿用库中明文
	keep := `{"endpoint":"s3.example.com","region":"us-east-1","bucket":"mybucket","access_key_id":"AKIDEXAMPLE0000","secret_access_key":"****xxxx","path_style":"true","presign_domain":"https://s3.img.li"}`
	got, err := svc.UpdatePolicy(p.ID, PolicyPatch{Config: strP(keep)})
	if err != nil {
		t.Fatalf("retain UpdatePolicy: %v", err)
	}
	if got.Config["secret_access_key"] != "SECRETxxxx" {
		t.Errorf("secret 应保留明文, got %q", got.Config["secret_access_key"])
	}
}
