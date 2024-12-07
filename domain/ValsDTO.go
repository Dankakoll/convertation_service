package domain

//Сущность валюты, хранится в бд
type ValsDTO struct {
	Date      string `redis:"Date" json:"date"`
	Source    string `redis:"Source" json:"-"`
	Code      string `redis:"Code" json:"Code"`
	Name      string `redis:"Name" json:"Name"`
	RatioBuy  string `redis:"RatioBuy" json:"RatioBuy"`
	RatioSell string `redis:"RatioSell" json:"RatioSell"`
}

//Приведение к сущности валюты
func ToValsDTO(Date string, source string, code string, name string, RatioBuy string, RatioSell string) ValsDTO {
	vals := new(ValsDTO)
	vals.Date = Date
	vals.Source = source
	vals.Code = code
	vals.Name = name
	vals.RatioBuy = RatioBuy
	vals.RatioSell = RatioSell
	return *vals
}
