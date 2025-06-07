package models

// Record 测土配肥记录模型
type Soil struct {
	Id                int     `json:"id"`
	UserId            int     `json:"userId"`
	AddNumber         int     `json:"addNumber"`
	Timestamp         string  `json:"timestamp"`
	Location          string  `json:"location"`
	Crop              string  `json:"crop"`
	PlotSize          float64 `json:"plotSize"`
	AverageYield      float64 `json:"averageYield"`
	OrganicFertilizer struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	} `json:"organicFertilizer"`
	FertilizerDemand struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"fertilizerDemand"`
	TotalSupply struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"totalSupply"`
	Supplement struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"supplement"`
	NitrogenReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"nitrogenReplenish"`
	PhosphorusReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"phosphorusReplenish"`
	PotassiumReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"potassiumReplenish"`
	NitrogenBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"nitrogenBasic"`
	PhosphorusBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"phosphorusBasic"`
	PotassiumBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"potassiumBasic"`
	CustomRatios string `json:"customRatios"`
}
