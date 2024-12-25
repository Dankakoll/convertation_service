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
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Стандартный курс для GetAll
const defaultSource = "RU"
const SourceRU = "RU"
const SourceTH = "TH"

// Логгер для API
var logger = log.New(os.Stdout, "API ", log.LstdFlags|log.Lshortfile)

// Сервис парсера
type ParseService interface {
	//В зависимости от тела ответа и источника идет приведение к структурам пакета domain
	// Возвращает ненулевую ошибку если тело пусто или такого источника нет
	Parse(source string, body []byte) (interface{}, error)
}

// Хендлер парсеров тел источника
type ParseHandler struct {
	Service ParseService
}

// Создание хендлера парсера по ссылкам источника. Нужна реализация интерфейса ParseService
func NewParseHandler(svc ParseService) *ParseHandler {
	return &ParseHandler{svc}
}

type FetcherService interface {
	// GetCurrfromSource отправляет GET-запрос по ссылке для конкретного источника и конкретной валюты (если нужно, добавить ключи доступа).
	// Возвращает ненулевую ошибку при получении статуса запроса не OK
	GetCurrfromSource(source string, curr string, sourceKeys map[string]string, sourceLinks map[string]string) (body []byte, err error)
}

// Хендлер запросов по ссылкам источника
type Fetcher struct {
	Service FetcherService
}

// Создание хендлера запросов по ссылкам источника. Нужна реализация интерфейса GetReqService
func NewGetFetcher(svc FetcherService) *Fetcher {
	return &Fetcher{svc}
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

func NewAPI(dbLink string, sourceKeys map[string]string, sourceLinks map[string]string, timeout int, timeLoc *time.Location, mainCtx context.Context) *API {
	//Инициализация бд
	opt, err := redis.ParseURL(dbLink)
	if err != nil {
		logger.Fatal("Wrong link provided")
	}
	//Клиент базы данных
	DatabaseHandler := domain.NewDatabaseHandler(redisdb.NewCurrModelRepository(redis.NewClient(opt)))
	return &API{sourceKeys, sourceLinks, timeout, timeLoc, mainCtx, DatabaseHandler}
}

func (a *API) ExitConnectWithDb(mainCtx context.Context) error {
	return a.DatabaseHandler.Service.Close(mainCtx)
}

// Инициализация сервисов DatabaseService , ParseService
// Возвращает ошибку, если источник неверен

func (a *API) initParseHandler(source string) (ParseHandler *ParseHandler, err error) {
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
	//Парсер
	ParseHandler = NewParseHandler(parser.NewParser(datatype))

	return ParseHandler, nil
}

// Обновление уже существующих валют.
// Возвращает ошибку если есть проблемы с подключением к БД или запрос к источнику вернул статус не OK
func (a *API) UpdateAllInSource(source string, timeLoc *time.Location, timeToUpdate time.Time) (err error) {
	//Поиск данных, нахождение ненулевой даты последнего обновления, парсинг и запись в бд
	defaultMessage := "When updating all currencys error occured in method %s. Error: %s"
	//Поиск данных
	var DBDTO []domain.CurrModel
	DBDTO, err = a.DatabaseHandler.Service.GetAllBySource(a.mainCtx, source)
	if err != nil {
		logger.Printf(defaultMessage, "DatabaseService.GetAllBySource", err.Error())
		return err
	}

	//Поиск ненулевой даты для сверки на обновление ( на случай ошибки в бд)
	//Если не нашли, то берем предыдущий день
	var currTime time.Time
	for _, v := range DBDTO {
		if len(v.Date) != 0 {
			currTime, err = time.Parse(time.DateOnly, v.Date)
			if err != nil {

				return err
			}
			break
		}
	}
	//Если даты нет
	if currTime.Year() == 0 {
		currTime = time.Now().In(timeLoc).AddDate(0, 0, -1)
	}

	//Обновление всех данных
	for _, v := range DBDTO {
		err = a.NewOrUpdateCurr(source, v.Code, currTime)
		if err != nil {
			logger.Printf(defaultMessage, "API.NewOrUpdateCurr", err.Error())
			return err
		}

	}
	return nil
}

// Получение новой валюты, если в запросе '/convert' приведена валюта, которой нет в базе.
// Или обновление уже существующих валют
// Возвращает ошибку если нет тела ответа, как правило при статусе не OK
// или нарушена целостность данных (например, когда структура ответа пуста)
func (a *API) NewOrUpdateCurr(source string, curr string, currencyDate time.Time) (err error) {
	//Cначала идет инициализация запросов,
	//затем получение информации из источника, затем парсинг и запись в бд данных

	//Инициализация сервисов
	ParseHandler, err := a.initParseHandler(source)
	if err != nil {
		return err
	}
	//Сервис отправки запросов
	GetFetcher := NewGetFetcher(fetcher.NewFetcher(currencyDate, a.timeLoc, a.timeout))
	//Получение тела ответа
	body, err := GetFetcher.Service.GetCurrfromSource(source, curr, a.sourceKeys, a.sourceLinks)
	if err != nil {
		return err
	}
	//Парсинг тела ответа
	newCurr, err := ParseHandler.Service.Parse(source, body)
	if err != nil {
		return err
	}
	//При разных источниках разные структуры ответа
	switch source {
	case SourceRU:
		{
			RUDTO := newCurr.(*domain.RUsourceDTO)
			splt := strings.Split(RUDTO.Date, ".")
			//Приведение даты к формату yyyy-mm-dd
			newDate := splt[2] + "-" + splt[1] + "-" + splt[0]
			//Случай получения пустых данных
			if len(newDate) == 0 {
				return errors.New("parsed nil data from source RU for curr " + curr + ". abort")
			}
			for _, curr := range RUDTO.Valute {
				a.DatabaseHandler.Service.Store(a.mainCtx, domain.ToCurrModel(
					newDate, SourceRU,
					curr.CharCode,
					curr.Name,
					curr.VunitRate, curr.VunitRate))
			}
			return nil

		}
	case SourceTH:
		{

			THDTO := newCurr.(domain.THsourceDTODataDetail)
			rg := regexp.MustCompile("[0-9]+")
			rgS := rg.FindAllString(THDTO.CurrencyNameEng, -1)
			initRateBuy, err := strconv.ParseFloat(strings.Replace(THDTO.BuyingTransfer, ",", ".", 1), 64)
			if err != nil {
				return errors.New("wrong RatioBuy in source TH for curr " + curr + ". Abort")
			}
			initRateSell, err := strconv.ParseFloat(strings.Replace(THDTO.Selling, ",", ".", 1), 64)
			if err != nil {
				return errors.New("wrong RatioSell in source TH " + curr + ". Abort")
			}
			//Данные в этом источнике могут иметь отношение на определенный номинал. Далее идет нормализация
			//(соотношение 1 бата к единице искомой валюты)
			if len(rgS) != 0 {
				amount, _ := strconv.ParseInt(rgS[0], 10, 16)
				THDTO.BuyingTransfer = strconv.FormatFloat(initRateBuy/float64(amount), 'f', 5, 64)
				THDTO.Selling = strconv.FormatFloat(initRateSell/float64(amount), 'f', 5, 64)
			}
			//Случай получения пустых данных
			if len(THDTO.Period) == 0 {
				return errors.New("parsed nil data from source TH. abort")
			}
			a.DatabaseHandler.Service.Store(a.mainCtx,
				domain.ToCurrModel(THDTO.Period, SourceTH,
					THDTO.CurrencyID, THDTO.CurrencyNameEng,
					THDTO.BuyingTransfer, THDTO.Selling))
		}
	default:
		return errors.New("wrong source " + source + " provided")
	}
	return nil
}

// Проверка правильности ввода запроса для метода `/convert`
// нужны параметры источника, первой и второй валюты, номинала и курса.
// Возвращает ошибку если какого то параметра не хватает или формат неверен (порядок не важен)
func (a *API) checkQuery(source string, first string, second string, amount string, course string) (err error) {
	var isPresented bool
	var exchangeTypes = []string{"buy", "sell"}
	for v := range a.sourceKeys {
		if v == source {
			isPresented = true
			break
		}
	}
	//Неверно указан источник из принимаемых
	if !isPresented {
		return errors.New("wrong source " + source + " provided")
	}
	isPresented = false
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
	for _, v := range exchangeTypes {
		if v == course {
			isPresented = true
			break
		}

	}
	//Неверно указан курс
	if !isPresented {
		return errors.New("wrong course provided: " + course)
	}
	return nil
}

// Проверка если курс валюты совпадает с курсом перевода источника или такой валюты нет.
// Возвращает ошибку метода NewOrUpdateCurr, если произошла ошибка поиска валюты в источнике
// или базы данных, если нет связи с бд или произошло непреднамеренное отключение
func (a *API) checkNameFromSource(source string, name string, ctx context.Context) (nameModel domain.CurrModel, err error) {
	//Проверка на курс источника
	if strings.Contains(name, source) {
		date := time.Now().Format(time.DateOnly)
		nameModel = domain.ToCurrModel(date, source, name, name, "1.0", "1.0")
	}
	//Поиск записи
	nameModel, err = a.DatabaseHandler.Service.GetBySourceAndKey(ctx, source, name)
	if err != nil {
		return domain.CurrModel{}, err
	}
	// Поиск записи в неподдерживаемых валютах
	unsupNameModel, err := a.DatabaseHandler.Service.GetBySourceAndKey(ctx, "UNSUP"+source, name)
	if err != nil {
		return domain.CurrModel{}, err
	}
	//Если нет
	if len(unsupNameModel.Name) != 0 {
		return domain.CurrModel{}, errors.New("unsupported currency " + name + " for source " + source)
	} else if len(nameModel.Name) == 0 {
		checkInDefSource, err := a.DatabaseHandler.Service.GetBySourceAndKey(ctx, defaultSource, name)
		if err != nil {
			return domain.CurrModel{}, err
		}
		if (len(checkInDefSource.Name)) == 0 {
			//Если в списке источника ЦБ РФ нет
			logger.Printf("Wrong currency name")
			return domain.CurrModel{}, errors.New("wrong currency name " + name)
		} else {
			//Добавление новой валюты
			err = a.NewOrUpdateCurr(source, name, time.Now())
			if err != nil {
				if err.Error() == "Invalid Currency" {
					a.DatabaseHandler.Service.Store(ctx, domain.ToCurrModel("", "UNSUP"+source, name, "Unsupported", "", ""))
				}
				logger.Printf("Cannot find data for this curr %s. Error: %e", name, err)
				return domain.CurrModel{}, err
			}
		}
	}
	return nameModel, nil
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
	defaultMessage := "In Convert error occured in method %s. Error: %s"
	defaulterr := errors.New("cannot provide now convert. try again later")
	//Проверка на правильность ввода
	err = a.checkQuery(source, first, second, amount, exchange)
	if err != nil {
		return nil, err
	}
	firstDTO, err := a.checkNameFromSource(source, first, a.mainCtx)
	if err != nil {
		logger.Printf(defaultMessage, "API.checkNameFromSource", err.Error())
		return nil, err
	}
	secondDTO, err := a.checkNameFromSource(source, second, a.mainCtx)
	if err != nil {
		logger.Printf(defaultMessage, "API.checkNameFromSource", err.Error())
		return nil, err
	}
	amountParsed, err := strconv.ParseFloat(strings.Replace(amount, ",", ".", 1), 64)
	if err != nil {
		return nil, errors.New("wrong amount passed")
	}
	var firstRatio, secondRatio float64
	//Обработка выбора курса продажи или покупки
	switch exchange {
	case "buy":
		{
			firstRatio, err = strconv.ParseFloat(strings.Replace(firstDTO.RatioBuy, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("Lost value for currency %s. Check connect with client or integrity of DB", first)
				return nil, defaulterr
			}
			secondRatio, err = strconv.ParseFloat(strings.Replace(secondDTO.RatioBuy, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("Lost value for currency %s. Check connect with client or integrity of DB", second)
				return nil, defaulterr
			}
		}
	case "sell":
		{
			firstRatio, err = strconv.ParseFloat(strings.Replace(firstDTO.RatioSell, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("Lost value for currency %s. Check connect with client or integrity of DB", first)
				return nil, defaulterr
			}
			secondRatio, err = strconv.ParseFloat(strings.Replace(secondDTO.RatioSell, ",", ".", 1), 64)
			if err != nil {
				logger.Printf("Lost value for currency %s. Check connect with client or integrity of DB", second)
				return nil, defaulterr
			}
		}
	}
	convertedAmount := amountParsed * firstRatio / secondRatio
	var res ConvertResponse
	/*Приведение к виду ответа с данными*/
	res.Date = firstDTO.Date
	res.Source = firstDTO.Source
	res.First = firstDTO.Code
	res.Second = secondDTO.Code
	res.Exchange = exchange
	res.Amount = amount
	res.ConvertedAmount = strconv.FormatFloat(convertedAmount, 'f', 3, 64)

	return res, nil
}

// Метод  реализует запрос '/getAll'. Достает из бд данные о валютах источника. Возвращает ошибку если потеряно соединене с бд после 5 попыток
func (a *API) GetAll(source string) (ans []domain.CurrModel, err error) {

	if len(source) == 0 {
		source = defaultSource
	}
	sourceDTOs, err := a.DatabaseHandler.Service.GetAllBySource(a.mainCtx, source)
	if err != nil {
		logger.Printf("Check logs for DB. Error:%e", err)
		return nil, errors.New("when requesting  data from database error occured. Try again later")
	}
	return sourceDTOs, nil
}
