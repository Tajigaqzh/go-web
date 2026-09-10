package model

type Material struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Key         string `json:"key" gorm:"uniqueIndex;size:64;not null"`
	Title       string `json:"title" gorm:"size:128;not null"`
	Kind        string `json:"kind" gorm:"size:32;not null"`
	PreviewURL  string `json:"preview_url" gorm:"size:512"`
	MinVipLevel int    `json:"min_vip_level"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`
}

func ListMaterials(vipLevel int) ([]Material, error) {
	var list []Material
	err := DB.Where("enabled = ? AND min_vip_level <= ?", true, vipLevel).
		Order("id asc").
		Find(&list).Error
	return list, err
}

func (m *Material) Insert() error {
	return DB.Create(m).Error
}

func seedMaterials() error {
	var n int64
	if err := DB.Model(&Material{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	defaults := []Material{
		{Key: "rect", Title: "矩形", Kind: "rect", MinVipLevel: VipFree, Enabled: true},
		{Key: "ellipse", Title: "椭圆", Kind: "ellipse", MinVipLevel: VipFree, Enabled: true},
		{Key: "text", Title: "文本", Kind: "text", MinVipLevel: VipFree, Enabled: true},
		{Key: "line", Title: "线条", Kind: "line", MinVipLevel: VipFree, Enabled: true},
		{Key: "image", Title: "图片", Kind: "image", MinVipLevel: VipFree, Enabled: true},
		{Key: "ai-style", Title: "AI 风格素材", Kind: "image", MinVipLevel: Vip1, Enabled: true},
		{Key: "pro-pack", Title: "高级素材包", Kind: "image", MinVipLevel: Vip2, Enabled: true},
	}
	return DB.Create(&defaults).Error
}
