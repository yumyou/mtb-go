package models

// IrrigationData 灌溉数据记录模型
type IrrigationData struct {
	ID                  int64   `json:"id"`
	UserID              int     `json:"user_id"`
	Timestamp           string  `json:"timestamp"`
	PlotSize            float64 `json:"plotSize"`
	MoisturePoints      string  `json:"moisturePoints"`
	IrrigationMode      string  `json:"irrigationMode"`
	CustomEfficiency    float64 `json:"customEfficiency"`
	FlowRate            float64 `json:"flowRate"`
	CropType            string  `json:"cropType"`
	CustomDepth         float64 `json:"customDepth"`
	OptimalMoisture     float64 `json:"optimalMoisture"`
	SoilType            string  `json:"soilType"`
	CustomFieldCapacity float64 `json:"customFieldCapacity"`
	SoilDensity         float64 `json:"soilDensity"`
	WaterAmount         float64 `json:"waterAmount"`
	IrrigationTime      string  `json:"irrigationTime"`
	FertilizerTankSize  string  `json:"fertilizerTankSize"`
	FertilizerStartTime string  `json:"fertilizerStartTime"`
	FertilizerFlowRate  float64 `json:"fertilizerFlowRate"`
	FertilizerTotalTime string  `json:"fertilizerTotalTime"`
	Negative            int     `json:"negative"`
}

// AreaData 区域数据模型，用于接收请求中的区域数据
type AreaData struct {
	AreaId              int             `json:"areaId"`
	WaterAmount         float64         `json:"waterAmount"`
	IrrigationTime      string          `json:"irrigationTime"`
	FertilizerFlowRate  float64         `json:"fertilizerFlowRate"`
	FertilizerStartTime string          `json:"fertilizerStartTime"`
	FertilizerTotalTime string          `json:"fertilizerTotalTime"`
	Negative            bool            `json:"negative"`
	TankSize            float64         `json:"tankSize"`
	WaterFlowRate       float64         `json:"waterFlowRate"`
	TankSizeName        string          `json:"tankSizeName"`
	MoisturePoints      []MoisturePoint `json:"moisturePoints"`
	PlotSize            float64         `json:"plotSize"`
	FlowRate            float64         `json:"flowRate"`
}

// WaterRecord 水记录模型
type WaterRecord struct {
	ID              int         `json:"id"`
	UserID          int         `json:"user_id"`
	IrrigationMode  string      `json:"irrigationMode"`
	Efficiency      float64     `json:"efficiency"`
	CropType        string      `json:"cropType"`
	Depth           float64     `json:"depth"`
	OptimalMoisture float64     `json:"optimalMoisture"`
	SoilType        string      `json:"soilType"`
	FieldCapacity   float64     `json:"fieldCapacity"`
	SoilDensity     float64     `json:"soilDensity"`
	CreatedAt       string      `json:"created_at"`
	Areas           []WaterArea `json:"areas"`
}

// WaterArea 水区域模型
type WaterArea struct {
	ID             int     `json:"id"`
	RecordID       int     `json:"record_id"`
	PlotSize       float64 `json:"plotSize"`
	WaterFlowRate  float64 `json:"waterFlowRate"`
	TankSize       float64 `json:"tankSize"`
	WaterAmount    float64 `json:"waterAmount"`
	IrrigationTime string  `json:"irrigationTime"`

	FertilizerStartTime string `json:"fertilizerStartTime"`
	FertilizerTotalTime string `json:"fertilizerTotalTime"`
	FertilizerFlowRate  string `json:"fertilizerFlowRate"`
	MoisturePoints      string `json:"moisturePoints"`
	Negative            bool   `json:"negative"`
}

// MoisturePoint 水分点模型
type MoisturePoint struct {
	Value float64 `json:"value"`
}
