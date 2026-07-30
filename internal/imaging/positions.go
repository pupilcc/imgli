package imaging

// Positions 九宫格 watermark position 合法枚举（tl…br），
// 供站点 processing、用户 preferences 与烧录管线共用。
var Positions = map[string]bool{
	"tl": true, "tc": true, "tr": true,
	"ml": true, "mc": true, "mr": true,
	"bl": true, "bc": true, "br": true,
}
