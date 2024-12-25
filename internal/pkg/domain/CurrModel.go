package domain

import "context"

//Сущность валюты, хранится в бд
type CurrModel struct {
	// Дата получения валюты
	Date string `redis:"Date" json:"date"`
	//Источник
	Source string `redis:"Source" json:"-"`
	//Код валюты
	Code string `redis:"Code" json:"Code"`
	//Полное название на языке из источника
	Name string `redis:"Name" json:"Name"`
	//Курс покупки
	RatioBuy string `redis:"RatioBuy" json:"RatioBuy"`
	//Курс продажи
	RatioSell string `redis:"RatioSell" json:"RatioSell"`
}

//Приведение к сущности валюты
func ToCurrModel(Date string, source string, code string, name string, RatioBuy string, RatioSell string) CurrModel {
	CurrModel := new(CurrModel)
	CurrModel.Date = Date
	CurrModel.Source = source
	CurrModel.Code = code
	CurrModel.Name = name
	CurrModel.RatioBuy = RatioBuy
	CurrModel.RatioSell = RatioSell
	return *CurrModel
}

// Сервис бд
type DatabaseService interface {
	// Получение данных по источнику и коду валюты. Возвращает сущность валюты пакета domain
	// и ненулевую ошибку при отключении от бд
	GetBySourceAndKey(ctx context.Context, source string, key string) (res CurrModel, err error)
	// Получение данных по источнику. Возвращает сущности валюты пакета domain
	// и ненулевую ошибку при отключении от бд
	GetAllBySource(ctx context.Context, source string) (res []CurrModel, err error)
	// Запись приведенных данных к сущности пакета domain.Возврашает ненулевую ошибку при отключении от бд
	Store(ctx context.Context, curr CurrModel) (err error)
	//Закрытие подключения к бд
	Close(ctx context.Context) (err error)
}

// Хендлер базы данных
type DatabaseHandler struct {
	Service DatabaseService
}

// Создание хендлера бд. Нужна реализация интерфейса DatabaseService
func NewDatabaseHandler(svc DatabaseService) *DatabaseHandler {
	return &DatabaseHandler{svc}
}
