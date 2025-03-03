// api реализует бизнес-логику для пакета handler. Требуется подключение к бд, иначе ответ в handler будет пуст
package api

import (
	"context"
	"errors"
	"log"
	"main/internal/pkg/domain"
	"main/internal/pkg/services/fetcher"
	"main/internal/pkg/services/parser"
	"main/internal/pkg/services/repo/redisdb"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Стандартный курс для GetAll
const defaultSource = "RU"
const SourceRU = "RU"
const SourceTH = "TH"
const SourceCurrNameRU = "RUB"
const SourceCurrNameTH = "THB"

// Логгер для API
var logger = log.New(os.Stdout, "API ", log.LstdFlags|log.Lshortfile)

// Сервис парсера
type ParseService interface {
	//В зависимости от тела ответа и источника идет приведение к структурам пакета domain
	// Возвращает ненулевую ошибку если тело пусто или такого источника нет
	Parse(source string, body []byte) (interface{}, error)
	//ПРиведение к модели, которая воспринимается бд
	ParseCurrstoDTO(newCurr interface{}, source string) (dom []domain.CurrModel, err error)
}

// Хендлер парсеров тел источника
type ParseHandler struct {
	Service ParseService
}

type FetcherService interface {
	// GetCurrfromSource отправляет GET-запрос по ссылке для конкретного источника и конкретной валюты (если нужно, добавить ключи доступа).
	// Возвращает ненулевую ошибку при получении статуса запроса не OK
	FetchAllfromSource(source string, sourceKeys map[string]string, sourceLinks map[string]string) (body []byte, err error)
}

// Хендлер запросов по ссылкам источника
type Fetcher struct {
	Service FetcherService
}

// API реализует запросы сервера, содержит данные о источниках, локации времени для сверки обновлений,
// и контекст для graceful shutdown

type API struct {
	sourceKeys      map[string]string
	sourceLinks     map[string]string
	timeout         int
	timeLoc         *time.Location
	mainCtx         context.Context
	DatabaseHandler *domain.DatabaseHandler
}

// API реализует запросы сервера, содержит данные о источниках, локации времени для сверки обновлений,
// и контекст для graceful shutdown

func NewAPI(dbLink string, sourceKeys map[string]string, sourceLinks map[string]string, timeout int, timeLoc *time.Location, mainCtx context.Context, DbMaxRetries int) (*API, error) {
	//Инициализация бд
	opt, err := redis.ParseURL(dbLink)
	if err != nil {
		logger.Printf("Wrong link provided.Check config.env")
		logger.Println(err.Error())
		return &API{}, err
	}
	//Клиент базы данных
	DatabaseHandler := domain.NewDatabaseHandler(redisdb.NewCurrModelRepository(redis.NewClient(opt), DbMaxRetries))
	//Сервис создания запросов
	return &API{sourceKeys, sourceLinks, timeout, timeLoc, mainCtx, DatabaseHandler}, nil
}

func (a *API) ExitConnectWithDb(mainCtx context.Context) error {
	return a.DatabaseHandler.Service.Close(mainCtx)
}

// Инициализация сервисов DatabaseService , ParseService
// Возвращает ошибку, если источник неверен

func (a *API) initParseHandler(source string) (Parser *ParseHandler, err error) {
	//Для каждого источника свой формат ответа
	var datatype string
	switch source {
	case SourceRU:
		datatype = "XML"
	case SourceTH:
		datatype = "JSON"
	default:
		return nil, errors.New("wrong source provided")
	}
	//Создание парсера через интерфейс
	Parser = &ParseHandler{parser.NewParser(datatype)}

	return Parser, nil
}

// Обновление уже существующих валют.
// Возвращает ошибку если есть проблемы с подключением к БД или запрос к источнику вернул статус не OK
func (a *API) UpdateAllInSource(source string, timeLoc *time.Location, timeToUpdate time.Time) (err error) {
	//Поиск данных, нахождение ненулевой даты последнего обновления, парсинг и запись в бд
	defaultMessage := "When updating all currencys error occured in method %s. Error: %s"
	//Ищем самое последнее обновление
	err = a.FetchAndUpdateCurrs(source, time.Now().In(timeLoc).AddDate(0, 0, -1))
	if err != nil {
		logger.Println(defaultMessage, "FetchAndUpdateCurrs", err.Error())
		return err
	}
	return nil
}

// Для метода update. Ищет и возращает время самой неактуальной записи в бд по источнику
func (a *API) GetDateFromSource(source string) (init_date time.Time, err error) {
	defaultMessage := "GetDateFromSource: "
	getAllAns, err := a.DatabaseHandler.Service.GetAllBySource(a.mainCtx, source)
	if err != nil {
		logger.Println(defaultMessage + "Internal db error. Err:" + err.Error())
		return init_date, err
	}
	init_date = time.Now()
	for _, c := range getAllAns {
		parsedDate, err := time.Parse(time.DateOnly, c.Date)
		if err != nil {
			logger.Println(defaultMessage + "Failed to parse date: " + c.Date)
			return init_date, err
		}
		if parsedDate.Before(init_date) {
			init_date = parsedDate
		}
	}
	return init_date, nil

}

// Получение новой валюты, если в запросе '/convert' приведена валюта, которой нет в базе.
// Или обновление уже существующих валют
// Возвращает ошибку если нет тела ответа, как правило при статусе не OK
// или нарушена целостность данных (например, когда структура ответа пуста)
func (a *API) FetchAndUpdateCurrs(source string, currencyDate time.Time) (err error) {
	defaultMessage := "FetchAndUpdateCurrs: "
	//Cначала идет инициализация запросов,
	//затем получение информации из источника, затем парсинг и запись в бд данных

	//Инициализация сервисов
	ParseHandler, err := a.initParseHandler(source)
	if err != nil {
		return err
	}
	//Сервис отправки запросов
	GetFetcher := &Fetcher{fetcher.NewFetcher(currencyDate, a.timeLoc, a.timeout)}
	//Получение тела ответа
	body, err := GetFetcher.Service.FetchAllfromSource(source, a.sourceKeys, a.sourceLinks)
	if err != nil {
		return err
	}
	//Парсинг тела ответа
	newCurr, err := ParseHandler.Service.Parse(source, body)
	if err != nil {
		return err
	}
	dto, err := ParseHandler.Service.ParseCurrstoDTO(newCurr, source)
	if err != nil {
		return err
	}
	for _, i := range dto {
		err = a.DatabaseHandler.Service.Store(a.mainCtx, i)
		if err != nil {
			logger.Println(defaultMessage + "Error adding data to db. Error:" + err.Error())
			return errors.New("cannot add data to db now")
		}
	}
	return nil
}

// Проверка правильности ввода запроса для метода `/convert`
// нужны параметры источника, первой и второй валюты, номинала и курса.
// Возвращает ошибку если какого то параметра не хватает или формат неверен (порядок не важен)
func (a *API) checkQuery(first string, second string, amount string, exchange string) (err error) {
	var exchangeTypes = []string{"buy", "sell"}
	// Неверно указан курс валют
	if !slices.Contains(exchangeTypes, exchange) {
		return errors.New("exchange type is wrong")
	}
	//Неверно указана валюта (должна иметь длину 3 и состоять из заглавных букв)
	if !(len(first) == 3 && regexp.MustCompile(`^[A-Z]+$`).MatchString(first)) {
		return errors.New("wrong first curr provided" + second)
	}
	if !(len(second) == 3 && regexp.MustCompile(`^[A-Z]+$`).MatchString(second)) {
		return errors.New("wrong second curr provided:" + second)
	}
	//Неверно указан номинал (должен иметь вид числа c плавающей точкой)
	if !(regexp.MustCompile("([0-9]*[.])?[0-9]+").MatchString(amount)) {
		return errors.New("wrong amount provided")
	}
	return nil
}

// Проверка если курс валюты совпадает с курсом перевода источника или такой валюты нет.
// Возвращает ошибку метода NewOrUpdateCurr, если произошла ошибка поиска валюты в источнике
// или базы данных, если нет связи с бд или произошло непреднамеренное отключение
func (a *API) checkNameFromSource(source string, name string, exchange string) (nameModel domain.CurrModel, nameRatio float64, err error) {
	defaultMessage := "checkNameFromSource :"
	//Проверка на курс источника
	if strings.Contains(name, source) {
		date := time.Now().Format(time.DateOnly)
		currname := ""
		switch name {
		case SourceCurrNameRU:
			currname = SourceCurrNameRU
		case SourceCurrNameTH:
			currname = SourceCurrNameTH
		default:
			err = errors.New("wrong currency name" + name)
			return domain.CurrModel{}, 1, err
		}
		nameModel = domain.ToCurrModel(date, source, currname, currname, "1.0", "1.0")
		return nameModel, 1, nil
	}
	//Поиск записи
	nameModel, err = a.DatabaseHandler.Service.GetBySourceAndKey(a.mainCtx, source, name)
	if err != nil {
		logger.Printf("%sCannot get %s model in source %s from DB . Check err: %e", defaultMessage, name, source, err)
		return domain.CurrModel{}, 1, err
	}
	//Если нет
	if len(nameModel.Name) == 0 {
		logger.Printf("%sWrong or lost currency %s in source %s ", defaultMessage, name, source)
		return domain.CurrModel{}, 1, errors.New("this currency " + name + " is unsupported or invalid for source " + source + ".")
	}
	switch exchange {
	case "buy":
		{
			nameRatio, err = strconv.ParseFloat(strings.Replace(nameModel.RatioBuy, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("%sLost value for currency %s. Check connect with client or integrity of DB. Error : %e ", defaultMessage, name, err)
				return domain.CurrModel{}, 1, errors.New("lost value for currency " + name + " check again later")
			}
		}
	case "sell":
		{
			nameRatio, err = strconv.ParseFloat(strings.Replace(nameModel.RatioSell, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("%sLost value for currency %s. Check connect with client or integrity of DB. Error: %e", defaultMessage, name, err)
				return domain.CurrModel{}, 1, errors.New("lost value for currency " + name + "check again later")
			}
		}
	}
	return nameModel, nameRatio, nil
}

// Тело ответа метода '/convert'
type ConvertResponse struct {
	Date            string `json:"date,omitempty"`
	Source          string `json:"source,omitempty"`
	First           string `json:"first_curr,omitempty"`
	Second          string `json:"second_curr,omitempty"`
	Exchange        string `json:"exchange,omitempty"`
	Amount          string `json:"amount,omitempty"`
	ConvertedAmount string `json:"converted_amount,omitempty"`
}

// Метод Сonvert реализует запрос '/convert'. Достает из бд данные о валютах источника, и
// при выбранном курсе перевода ( продажа/покупка), выводит тело ответа с данными о валютах и переведенном номинале
// Возвращает ошибку если неправильно введены параметры или проблема с бд
func (a *API) Convert(source string, first string, second string, amount string, exchange string) (data interface{}, err error) {
	defaultMessage := "In Convert error occured in method %s. Check logs"
	//Проверка на правильность ввода
	err = a.checkQuery(first, second, amount, exchange)
	if err != nil {
		return nil, err
	}
	firstDTO, firstRatio, err := a.checkNameFromSource(source, first, exchange)
	if err != nil {
		logger.Printf(defaultMessage, "checkNameFromSource")
		return nil, err
	}
	secondDTO, secondRatio, err := a.checkNameFromSource(source, second, exchange)
	if err != nil {
		logger.Printf(defaultMessage, "checkNameFromSource")
		return nil, err
	}
	amountParsed, err := strconv.ParseFloat(strings.Replace(amount, ",", ".", 1), 64)
	if err != nil {
		return nil, errors.New("wrong amount passed")
	}
	//Обработка выбора курса продажи или покупки
	convertedAmount := amountParsed * firstRatio / secondRatio
	var res ConvertResponse
	/*Приведение к виду ответа с данными*/
	res.Date = firstDTO.Date
	res.Source = firstDTO.Source
	res.First = firstDTO.Code
	res.Second = secondDTO.Code
	res.Exchange = exchange
	res.Amount = amount
	res.ConvertedAmount = strconv.FormatFloat(convertedAmount, 'f', 12, 64)

	return res, nil
}

// Метод  реализует запрос '/getAll'. Достает из бд данные о валютах источника. Возвращает ошибку если потеряно соединене с бд после 5 попыток
func (a *API) GetAll(source string) (ans []domain.CurrModel, err error) {
	defaultMessage := "GetAll: "
	if len(source) == 0 {
		source = defaultSource
	}
	sourceDTOs, err := a.DatabaseHandler.Service.GetAllBySource(a.mainCtx, source)
	if err != nil {
		logger.Printf(defaultMessage+"Check logs for DB. Error:%e", err)
		return nil, errors.New("when requesting  data from database error occured. Try again later")
	}
	if (len(sourceDTOs)) == 0 {
		return nil, errors.New("wrong source provided")
	}
	return sourceDTOs, nil
}
