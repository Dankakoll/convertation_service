package domain

import "context"

//Сущность валюты, хранится в бд
type CurrModel struct {
	// Дата получения валюты
	Date string `redis:"Date" json:"date"`
	//Источник
	Source string `redis:"Source" json:"-"`
	//Код валюты
	Code string `redis:"Code" json:"code"`
	//Полное название на языке из источника
	Name string `redis:"Name" json:"name"`
	//Курс покупки
	RatioBuy string `redis:"RatioBuy" json:"ratio_buy"`
	//Курс продажи
	RatioSell string `redis:"RatioSell" json:"ratio_sell"`
}

//Приведение к сущности валюты
func ToCurrModel(date string, source string, code string, name string, ratioBuy string, ratioSell string) CurrModel {
	return CurrModel{
		Date:      date,
		Source:    source,
		Code:      code,
		Name:      name,
		RatioBuy:  ratioBuy,
		RatioSell: ratioSell,
	}
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
	return &DatabaseHandler{Service: svc}
}
