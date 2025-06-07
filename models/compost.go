package models

// Fertilizer 肥料模型
type Fertilizer struct {
	Name     string  `json:"name"`
	Weight   float64 `json:"weight"`
	C        float64 `json:"c"`
	N        float64 `json:"n"`
	Moisture float64 `json:"moisture"`
	C_N      float64 `json:"c_n"`
}

// CompostHistory 堆肥历史记录模型
type CompostHistory struct {
	ID                  int          `json:"id"`
	NitrogenSourcesList []Fertilizer `json:"nitrogenSourcesList"`
	CarbonSourcesList   []Fertilizer `json:"carbonSourcesList"`
	AllVolume           float64      `json:"allVolume"`
	CNRatio             string       `json:"cNRatio"`
	Density             string       `json:"density"`
	WaterAdd            string       `json:"waterAdd"`
	CreatedAt           string       `json:"created_at"`
	UserID              int          `json:"user_id"`
}

// Source 源结构体，数值字段为 string 类型
type Source struct {
	Name            string `json:"name"`
	CarbonContent   string `json:"carbon_content"`
	NitrogenContent string `json:"nitrogen_content"`
	MoistureContent string `json:"moisture_content"`
	Density         string `json:"density"`
}

// Result 计算结果结构体
type Result struct {
	CNRatio         string `json:"cn_ratio"`
	Density         string `json:"density"`
	MoistureContent string `json:"moisture_content"`
}