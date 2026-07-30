package storage

import (
	"fmt"
	"net/url"
	"strings"
)

// Tier is the support level shown in admin UI; it does not change the serve hot path.
type Tier string

const (
	TierFirstClass  Tier = "first_class"
	TierSupported   Tier = "supported"
	TierCompat      Tier = "compat"
	TierMigrateOnly Tier = "migrate_only"
)

// Caps is the static capability profile of a driver (independent of one policy's config).
// Object CRUD is implied by the Driver interface and is not repeated here.
type Caps struct {
	Tier       Tier   `json:"tier"`
	SummaryKey string `json:"summary_key"`

	TransportTLSPreferred       bool `json:"transport_tls_preferred"`
	AllowsInsecure              bool `json:"allows_insecure"`
	RangeGet                    bool `json:"range_get"`
	ListPrefix                  bool `json:"list_prefix"`
	MultipartUpload             bool `json:"multipart_upload"`
	PublicCDNOffloadRecommended bool `json:"public_cdn_offload_recommended"`
	PrivatePresignCapable       bool `json:"private_presign_capable"`
	HotPathOK                   bool `json:"hot_path_ok"`

	FeatureLossKeys []string `json:"feature_loss_keys"`
}

// Effective is Caps plus per-policy configuration outcomes.
type Effective struct {
	TransportIsTLS              bool `json:"transport_is_tls"`
	PublicCDNRedirectConfigured bool `json:"public_cdn_redirect_configured"`
	PrivatePresignReady         bool `json:"private_presign_ready"`
}

// PolicyWarning is a structured advisory for admin API (backend is authoritative).
type PolicyWarning struct {
	Code       string `json:"code"`
	MessageKey string `json:"message_key"`
	Severity   string `json:"severity"` // warning | info
}

// CapabilityProvider is optional: a Driver may declare Caps; otherwise CapsForDriver is used.
type CapabilityProvider interface {
	Capabilities() Caps
}

// CapsForDriver returns the static profile for a known driver name.
func CapsForDriver(driver string) (Caps, error) {
	c, ok := capsByDriver[strings.ToLower(strings.TrimSpace(driver))]
	if !ok {
		return Caps{}, fmt.Errorf("storage: unknown driver %q", driver)
	}
	// Copy slice so callers cannot mutate the table.
	out := c
	if len(c.FeatureLossKeys) > 0 {
		out.FeatureLossKeys = append([]string(nil), c.FeatureLossKeys...)
	}
	return out, nil
}

// EffectiveFor builds Effective from a policy's driver + config + CDN domain.
// policyCDN and config are raw policy fields (config may be nil).
func EffectiveFor(driver string, config map[string]string, policyCDN string) (Effective, error) {
	caps, err := CapsForDriver(driver)
	if err != nil {
		return Effective{}, err
	}
	if config == nil {
		config = map[string]string{}
	}
	eff := Effective{
		TransportIsTLS:              transportIsTLS(driver, config, caps),
		PublicCDNRedirectConfigured: strings.TrimSpace(policyCDN) != "",
		PrivatePresignReady:         privatePresignReady(driver, config, caps),
	}
	return eff, nil
}

// WarningsFor returns advisory warnings for a policy (does not fail create/update).
func WarningsFor(driver string, config map[string]string, policyCDN string, enabled bool, caps Caps, eff Effective) []PolicyWarning {
	var w []PolicyWarning
	if strings.TrimSpace(policyCDN) != "" && !caps.PublicCDNOffloadRecommended {
		w = append(w, PolicyWarning{
			Code: "cdn_not_recommended", MessageKey: "adminB.warnCdnWithoutCap", Severity: "warning",
		})
	}
	if caps.PrivatePresignCapable && !eff.PrivatePresignReady {
		w = append(w, PolicyWarning{
			Code: "presign_unconfigured", MessageKey: "adminB.warnPresignUnconfigured", Severity: "info",
		})
	}
	if isRemoteDriver(driver) && !eff.TransportIsTLS {
		w = append(w, PolicyWarning{
			Code: "insecure_transport", MessageKey: "adminB.warnInsecureTransport", Severity: "warning",
		})
	}
	if enabled && caps.Tier == TierCompat {
		// Listed on every compat policy; doctor also aggregates site-wide compat_only.
		w = append(w, PolicyWarning{
			Code: "compat_tier", MessageKey: "adminB.warnCompatTier", Severity: "warning",
		})
	}
	return w
}

func isRemoteDriver(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "s3", "webdav", "ftp":
		return true
	default:
		return false
	}
}

func transportIsTLS(driver string, config map[string]string, caps Caps) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "local":
		return true
	case "s3", "webdav":
		ep := strings.TrimSpace(config["endpoint"])
		if ep == "" {
			return caps.TransportTLSPreferred
		}
		// Allow bare host: treat as non-TLS until scheme known; s3.New may prepend https.
		if !strings.Contains(ep, "://") {
			return true // s3 driver defaults to https when scheme omitted in New
		}
		u, err := url.Parse(ep)
		if err != nil {
			return false
		}
		return strings.EqualFold(u.Scheme, "https")
	case "ftp":
		if strings.EqualFold(strings.TrimSpace(config["allow_insecure"]), "true") {
			return false
		}
		// Default FTPS / TLS required.
		return true
	default:
		return caps.TransportTLSPreferred
	}
}

func privatePresignReady(driver string, config map[string]string, caps Caps) bool {
	if !caps.PrivatePresignCapable {
		return false
	}
	if strings.ToLower(strings.TrimSpace(driver)) != "s3" {
		return false
	}
	return strings.TrimSpace(config["presign_domain"]) != ""
}

var capsByDriver = map[string]Caps{
	"local": {
		Tier:                        TierFirstClass,
		SummaryKey:                  "storage.caps.summary.local",
		TransportTLSPreferred:       true,
		AllowsInsecure:              false,
		RangeGet:                    true,
		ListPrefix:                  false,
		MultipartUpload:             false,
		PublicCDNOffloadRecommended: false,
		PrivatePresignCapable:       false,
		HotPathOK:                   true,
		FeatureLossKeys: []string{
			"storage.loss.no_presign",
			"storage.loss.cdn_not_typical",
		},
	},
	"s3": {
		Tier:                        TierFirstClass,
		SummaryKey:                  "storage.caps.summary.s3",
		TransportTLSPreferred:       true,
		AllowsInsecure:              false,
		RangeGet:                    true,
		ListPrefix:                  false,
		MultipartUpload:             false,
		PublicCDNOffloadRecommended: true,
		PrivatePresignCapable:       true,
		HotPathOK:                   true,
		FeatureLossKeys:             nil,
	},
	"webdav": {
		Tier:                        TierSupported,
		SummaryKey:                  "storage.caps.summary.webdav",
		TransportTLSPreferred:       true,
		AllowsInsecure:              false,
		RangeGet:                    true,
		ListPrefix:                  false,
		MultipartUpload:             false,
		PublicCDNOffloadRecommended: false,
		PrivatePresignCapable:       false,
		HotPathOK:                   true,
		FeatureLossKeys: []string{
			"storage.loss.no_presign",
			"storage.loss.cdn_not_typical",
			"storage.loss.vendor_semantics",
		},
	},
	"ftp": {
		Tier:                        TierCompat,
		SummaryKey:                  "storage.caps.summary.ftp",
		TransportTLSPreferred:       true,
		AllowsInsecure:              true,
		RangeGet:                    false,
		ListPrefix:                  false,
		MultipartUpload:             false,
		PublicCDNOffloadRecommended: false,
		PrivatePresignCapable:       false,
		HotPathOK:                   false,
		FeatureLossKeys: []string{
			"storage.loss.no_presign",
			"storage.loss.no_cdn_offload",
			"storage.loss.cdn_not_typical",
			"storage.loss.hot_path",
			"storage.loss.ftp_security",
			"storage.loss.ftp_reliability",
		},
	},
}

// ValidateCDNDomain checks policy CDN domain. Empty is OK.
// Allows http(s) with host; forbids userinfo, query, fragment; path prefix allowed.
func ValidateCDNDomain(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("storage: invalid cdn_domain")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("storage: invalid cdn_domain")
	}
	return nil
}
