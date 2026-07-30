package adminsvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/yixian-huang/imgli/internal/model"
	"github.com/yixian-huang/imgli/internal/storage"
	"github.com/yixian-huang/imgli/internal/storage/ftp"
	"github.com/yixian-huang/imgli/internal/storage/s3"
	"github.com/yixian-huang/imgli/internal/storage/webdav"
)

var (
	// ErrPolicyNameInvalid 策略名需 1-64 个字符（TrimSpace 后）。
	ErrPolicyNameInvalid = errors.New("策略名需 1-64 个字符")
	// ErrDriverUnsupported driver 仅支持 local|s3|webdav|ftp。
	ErrDriverUnsupported = errors.New("driver 仅支持 local|s3|webdav|ftp")
	// ErrBadConfig 驱动 config 校验失败（local 缺 root / s3 缺必填字段等）。
	ErrBadConfig = errors.New("config 缺少非空 root")
	// ErrPolicyInUse 仍被 files 引用的策略不可删除（改用 enabled=false 下线）。
	ErrPolicyInUse = errors.New("存储策略仍被文件引用，不可删除")
	// ErrPolicyInUseByGroup 策略仍被某用户组的 allowed_policy_ids 引用，不可删除。
	// allowed_policy_ids 是 JSON 数组列，FK 管不到，需服务层自查（RESTRICT 语义，同 ErrPolicyInUse）。
	ErrPolicyInUseByGroup = errors.New("存储策略仍被用户组引用，不可删除")
	// ErrPolicyNotFound 复用 groups.go 已定义的同名错误（策略不存在）。
)

// defaultPathTemplate 是②b 约定的默认存储路径模板，PathTemplate 留空时套用。
const defaultPathTemplate = "{Y}/{m}/{d}/{uniqid}.{ext}"

// PolicyRow 是列表项：存储策略 + 实时算出的引用文件数与已用字节。
type PolicyRow struct {
	Policy    model.StoragePolicy
	FileCount int64
	UsedBytes int64
}

// PolicyPatch 是 UpdatePolicy 的部分更新载荷：nil 字段保持不变。
// 不含 Driver——换驱动等价于建新策略（裁决 7），本切片仅 local 无需处理迁移。
// Config 为 JSON 编码的字符串（如 `{"root":"/data"}`），与 GET 响应的 config 字段
// 编码方式一致，patch 前在服务层解析校验。
type PolicyPatch struct {
	Name         *string
	Config       *string
	CDNDomain    *string
	PathTemplate *string
	Enabled      *bool
}

// validateName 校验策略名 1-64 字符（TrimSpace 后），返回规整后的名字。
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return "", ErrPolicyNameInvalid
	}
	return name, nil
}

// validateLocalConfig 校验 local 驱动的 config 含非空 root。
func validateLocalConfig(cfg map[string]string) error {
	if strings.TrimSpace(cfg["root"]) == "" {
		return ErrBadConfig
	}
	return nil
}

// validateS3Config 校验 s3 驱动必填字段、path_style 与可选 presign_domain。
func validateS3Config(cfg map[string]string) error {
	for _, k := range []string{"endpoint", "region", "bucket", "access_key_id", "secret_access_key"} {
		if strings.TrimSpace(cfg[k]) == "" {
			return ErrBadConfig
		}
	}
	switch cfg["path_style"] {
	case "", "true", "false":
	default:
		return ErrBadConfig
	}
	// presign_domain 可选;非空时必须是纯 origin 的 http(s) URL。带 path 会破坏
	// SigV4 的 canonical URI(签名覆盖 path),内联 userinfo 则是明文凭据回显面。
	// 非 ASCII 主机名拒绝:与 s3.New 一致——浏览器会转 punycode,我们无转换
	// 能力,放行后 New 失败会拖垮该策略下全部读写(不只是预签名)。
	if pd := strings.TrimSpace(cfg["presign_domain"]); pd != "" {
		u, err := url.Parse(pd)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ErrBadConfig
		}
		if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return ErrBadConfig
		}
		host := strings.ToLower(u.Host)
		for i := 0; i < len(host); i++ {
			if host[i] >= 0x80 {
				return ErrBadConfig
			}
		}
	}
	return nil
}

// validateWebDAVConfig 校验 webdav 驱动:endpoint 非空且为合法 http(s) URL;
// username/password 不强制(开放 WebDAV)。
func validateWebDAVConfig(cfg map[string]string) error {
	endpoint := strings.TrimSpace(cfg["endpoint"])
	if endpoint == "" {
		return ErrBadConfig
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrBadConfig
	}
	// endpoint 不得内联 userinfo(明文凭据会绕过 password 打码,经 DTO 回显;codex 终审)
	if u.User != nil {
		return ErrBadConfig
	}
	return nil
}

// validateFTPConfig 校验 ftp 兼容驱动：host（或 endpoint）必填；allow_insecure 仅 true/false/空。
func validateFTPConfig(cfg map[string]string) error {
	host := strings.TrimSpace(cfg["host"])
	if host == "" {
		ep := strings.TrimSpace(cfg["endpoint"])
		if ep == "" {
			return ErrBadConfig
		}
	}
	switch strings.ToLower(strings.TrimSpace(cfg["allow_insecure"])) {
	case "", "true", "false":
	default:
		return ErrBadConfig
	}
	if p := strings.TrimSpace(cfg["port"]); p != "" {
		// 粗校验：1–65535
		n := 0
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return ErrBadConfig
			}
			n = n*10 + int(p[i]-'0')
			if n > 65535 {
				return ErrBadConfig
			}
		}
		if n == 0 {
			return ErrBadConfig
		}
	}
	// 试构造确保 New 可解析（不拨号）
	if _, err := ftp.New(cfg); err != nil {
		return ErrBadConfig
	}
	return nil
}

// validateCDNDomain wraps storage.ValidateCDNDomain as ErrBadConfig.
func validateCDNDomain(raw string) error {
	if err := storage.ValidateCDNDomain(raw); err != nil {
		return ErrBadConfig
	}
	return nil
}

// validateDriverConfig 按驱动类型校验 config。
func validateDriverConfig(driver string, cfg map[string]string) error {
	switch driver {
	case "local":
		return validateLocalConfig(cfg)
	case "s3":
		return validateS3Config(cfg)
	case "webdav":
		return validateWebDAVConfig(cfg)
	case "ftp":
		return validateFTPConfig(cfg)
	default:
		return ErrDriverUnsupported
	}
}

// PolicyCapsBundle is attached to admin policy DTOs.
type PolicyCapsBundle struct {
	Tier      storage.Tier
	Caps      storage.Caps
	Effective storage.Effective
	Warnings  []storage.PolicyWarning
}

// CapsBundleFor builds tier/caps/effective/warnings for a storage policy.
func CapsBundleFor(p *model.StoragePolicy) (PolicyCapsBundle, error) {
	caps, err := storage.CapsForDriver(p.Driver)
	if err != nil {
		return PolicyCapsBundle{}, err
	}
	eff, err := storage.EffectiveFor(p.Driver, p.Config, p.CDNDomain)
	if err != nil {
		return PolicyCapsBundle{}, err
	}
	return PolicyCapsBundle{
		Tier:      caps.Tier,
		Caps:      caps,
		Effective: eff,
		Warnings:  storage.WarningsFor(p.Driver, p.Config, p.CDNDomain, p.Enabled, caps, eff),
	}, nil
}

// parseDriverConfig 把 PATCH 传入的 JSON 字符串解析为 map 并按驱动校验。
func parseDriverConfig(driver, raw string) (map[string]string, error) {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, ErrBadConfig
	}
	if err := validateDriverConfig(driver, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ListPolicies 按 id 升序返回全部存储策略，含每条策略实时算出的引用文件数（FileCount）
// 与已用字节（UsedBytes=SUM(files.size)）。策略数量少，不分页。
func (s *Service) ListPolicies() ([]PolicyRow, error) {
	var policies []model.StoragePolicy
	if err := s.db.Order("id").Find(&policies).Error; err != nil {
		return nil, err
	}
	rows := make([]PolicyRow, 0, len(policies))
	for i := range policies {
		var fc int64
		if err := s.db.Model(&model.File{}).Where("storage_policy_id = ?", policies[i].ID).Count(&fc).Error; err != nil {
			return nil, err
		}
		var used int64
		if err := s.db.Model(&model.File{}).Where("storage_policy_id = ?", policies[i].ID).
			Select("COALESCE(SUM(size),0)").Scan(&used).Error; err != nil {
			return nil, err
		}
		rows = append(rows, PolicyRow{Policy: policies[i], FileCount: fc, UsedBytes: used})
	}
	return rows, nil
}

// PolicyByID 按 id 取单个存储策略（不存在 → ErrPolicyNotFound）；供 handler 侧写操作前
// 取展示字段（如 audit 里的 name），同 groups.go 的 GroupByID 用法。
func (s *Service) PolicyByID(id uint64) (*model.StoragePolicy, error) {
	var p model.StoragePolicy
	if err := s.db.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return &p, nil
}

// CreatePolicy 创建存储策略。校验：Name 非空且 ≤64；Driver 仅 local|s3|webdav|ftp；
// 各驱动 Config 按 validateDriverConfig 校验；PathTemplate 留空时套用②b 默认模板。
func (s *Service) CreatePolicy(p *model.StoragePolicy) error {
	name, err := validateName(p.Name)
	if err != nil {
		return err
	}
	if err := validateDriverConfig(p.Driver, p.Config); err != nil {
		return err
	}
	if err := validateCDNDomain(p.CDNDomain); err != nil {
		return err
	}
	p.Name = name
	pt := strings.TrimSpace(p.PathTemplate)
	if pt == "" {
		pt = defaultPathTemplate
	}
	p.PathTemplate = pt

	// Enabled 带 `gorm:"default:true"`。GORM Create()对 bool 零值(false)+可解析字面量默认值
	// 的字段有既定行为：只要 Go 侧值是该类型零值(false)，创建时一律回填默认值 true（连同改写
	// 调用方传入的结构体字段本身，Select() 无法绕过——该替换发生在字段级 isZero 判定，先于
	// Select 的列过滤生效）。调用方（handler 层）已用 *bool 请求字段消解了"未设置"与"显式
	// false"的二义（未设置时默认填 true 再传入），故此处 p.Enabled==false 到达时即代表调用方
	// 真实想要禁用——在 Create 前记录意图，Create 后若被 GORM 静默改回 true，用一条单列
	// UPDATE 纠正。这不是"先读后写"竞态：该行刚插入、id 尚未对外暴露，不存在并发覆盖窗口。
	// 注意：struct 路径的 Updates 同样会忽略零值字段（同款 isZero 问题），必须用
	// Update("enabled", false) 单列/map 形式——enabled 是无 serializer 的普通 bool 列，安全。
	wantDisabled := !p.Enabled
	if err := s.db.Create(p).Error; err != nil {
		return err
	}
	if wantDisabled {
		if err := s.db.Model(p).Update("enabled", false).Error; err != nil {
			return err
		}
		p.Enabled = false
	}
	return nil
}

// UpdatePolicy 部分更新存储策略。driver 不可改（裁决 7：换驱动=建新策略，PolicyPatch 无此字段）。
// 写回只 Select 本次实际 patch 到的列（避免并发下的 lost update，同 groups.go UpdateGroup
// 的既有写法）；末尾 RowsAffected 门禁：First 与 Updates 之间若该策略被并发删除，报
// ErrPolicyNotFound 而非用陈旧内存对象冒充更新成功。
func (s *Service) UpdatePolicy(id uint64, patch PolicyPatch) (*model.StoragePolicy, error) {
	var p model.StoragePolicy
	if err := s.db.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}

	var cols []string
	if patch.Name != nil {
		name, err := validateName(*patch.Name)
		if err != nil {
			return nil, err
		}
		p.Name = name
		cols = append(cols, "name")
	}
	if patch.Config != nil {
		cfg, err := parseDriverConfig(p.Driver, *patch.Config)
		if err != nil {
			return nil, err
		}
		// s3 密钥掩码保留「改指向即失效」:secret 以 **** 开头时,仅当 endpoint/region/
		// bucket/access_key_id/presign_domain 均未变才沿用库中明文,否则 ErrBadConfig
		// (防改指向窃旧密钥)。presign_domain 同属指向类字段——它决定签名被签给哪个
		// 主机,改它等于改签名的受益方。
		if p.Driver == "s3" && strings.HasPrefix(cfg["secret_access_key"], "****") {
			old := p.Config
			if cfg["endpoint"] != old["endpoint"] || cfg["region"] != old["region"] ||
				cfg["bucket"] != old["bucket"] || cfg["access_key_id"] != old["access_key_id"] ||
				cfg["presign_domain"] != old["presign_domain"] {
				return nil, ErrBadConfig
			}
			cfg["secret_access_key"] = old["secret_access_key"]
		}
		// webdav 密码掩码保留「改指向即失效」:password 以 **** 开头时,仅当
		// endpoint/username 均未变才沿用库中明文,否则 ErrBadConfig。
		if p.Driver == "webdav" && strings.HasPrefix(cfg["password"], "****") {
			old := p.Config
			if cfg["endpoint"] != old["endpoint"] || cfg["username"] != old["username"] {
				return nil, ErrBadConfig
			}
			cfg["password"] = old["password"]
		}
		// ftp 同 webdav 掩码语义：改 host/port/username 必须重交 password。
		if p.Driver == "ftp" && strings.HasPrefix(cfg["password"], "****") {
			old := p.Config
			if cfg["host"] != old["host"] || cfg["endpoint"] != old["endpoint"] ||
				cfg["port"] != old["port"] || cfg["username"] != old["username"] {
				return nil, ErrBadConfig
			}
			cfg["password"] = old["password"]
		}
		p.Config = cfg
		cols = append(cols, "config")
	}
	if patch.CDNDomain != nil {
		if err := validateCDNDomain(*patch.CDNDomain); err != nil {
			return nil, err
		}
		p.CDNDomain = *patch.CDNDomain
		cols = append(cols, "cdn_domain")
	}
	if patch.PathTemplate != nil {
		pt := strings.TrimSpace(*patch.PathTemplate)
		if pt == "" {
			pt = defaultPathTemplate
		}
		p.PathTemplate = pt
		cols = append(cols, "path_template")
	}
	if patch.Enabled != nil {
		p.Enabled = *patch.Enabled
		cols = append(cols, "enabled")
	}

	if len(cols) == 0 {
		return &p, nil
	}

	res := s.db.Model(&p).Select(cols).Updates(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrPolicyNotFound
	}
	return &p, nil
}

// DeletePolicy 删除存储策略。仍被 files 引用（COUNT>0）不可删（ErrPolicyInUse，改用
// enabled=false 下线）；仍被某用户组的 allowed_policy_ids 引用也不可删（ErrPolicyInUseByGroup——
// 该列是 JSON 数组，FK 管不到，需服务层自查，同为 RESTRICT 语义）；不存在 → ErrPolicyNotFound。
// 末尾显式 RowsAffected 门禁：First/引用检查与 Delete 之间若该策略被并发删除，Delete 匹配不到
// 行而 RowsAffected==0，此时必须报 ErrPolicyNotFound，不得让前面的检查都通过就冒充删除成功
// （同 groups.go DeleteGroup 的既有防假成功写法）。
func (s *Service) DeletePolicy(id uint64) error {
	var p model.StoragePolicy
	if err := s.db.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPolicyNotFound
		}
		return err
	}
	var n int64
	if err := s.db.Model(&model.File{}).Where("storage_policy_id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrPolicyInUse
	}
	// 组数量少，全量取回内存判断 allowed_policy_ids（JSON 列，SQL 不便按元素查）。
	var groups []model.UserGroup
	if err := s.db.Select("id", "allowed_policy_ids").Find(&groups).Error; err != nil {
		return err
	}
	for i := range groups {
		for _, pid := range groups[i].AllowedPolicyIDs {
			if pid == id {
				return ErrPolicyInUseByGroup
			}
		}
	}
	res := s.db.Delete(&model.StoragePolicy{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

// TestPolicy 对策略做写/读/删探针：local 直接测目录可写；s3/webdav/ftp 走驱动 Put/Open/Delete。
// 返回耗时(ms)。失败时返回描述性 error。
func (s *Service) TestPolicy(id uint64) (int64, error) {
	var p model.StoragePolicy
	if err := s.db.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrPolicyNotFound
		}
		return 0, err
	}
	switch p.Driver {
	case "local":
		root := strings.TrimSpace(p.Config["root"])
		if root == "" {
			return 0, ErrBadConfig
		}

		start := time.Now()
		if err := os.MkdirAll(root, 0o755); err != nil {
			return 0, fmt.Errorf("root 不可写: %w", err)
		}
		name := ".imgli-probe-" + randSuffix(8)
		path := filepath.Join(root, name)
		content := []byte(randSuffix(16))
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return 0, fmt.Errorf("写入探针文件失败: %w", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			os.Remove(path)
			return 0, fmt.Errorf("读回探针文件失败: %w", err)
		}
		if !bytes.Equal(got, content) {
			os.Remove(path)
			return 0, errors.New("探针文件内容比对不一致")
		}
		if err := os.Remove(path); err != nil {
			return 0, fmt.Errorf("删除探针文件失败: %w", err)
		}
		return time.Since(start).Milliseconds(), nil
	case "s3", "webdav", "ftp":
		var d interface {
			Put(context.Context, string, io.Reader) error
			Open(context.Context, string) (io.ReadSeekCloser, error)
			Delete(context.Context, string) error
		}
		var err error
		switch p.Driver {
		case "s3":
			d, err = s3.New(p.Config)
		case "webdav":
			d, err = webdav.New(p.Config)
		case "ftp":
			d, err = ftp.New(p.Config)
		}
		if err != nil {
			return 0, err
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		probeKey := ".imgli-probe-" + randSuffix(8)
		content := []byte(randSuffix(16))
		if err := d.Put(ctx, probeKey, bytes.NewReader(content)); err != nil {
			return 0, fmt.Errorf("写入探针对象失败: %w", err)
		}
		rc, err := d.Open(ctx, probeKey)
		if err != nil {
			_ = d.Delete(ctx, probeKey)
			return 0, fmt.Errorf("读回探针对象失败: %w", err)
		}
		got, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.Equal(got, content) {
			_ = d.Delete(ctx, probeKey)
			return 0, errors.New("探针对象内容比对不一致")
		}
		if err := d.Delete(ctx, probeKey); err != nil {
			return 0, fmt.Errorf("删除探针对象失败: %w", err)
		}
		return time.Since(start).Milliseconds(), nil
	default:
		return 0, ErrDriverUnsupported
	}
}

// randSuffix 生成 n 字节随机数的十六进制串，供探针文件名/内容使用。
func randSuffix(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
