// api реализует бизнес-логику для пакета handler. Требуется подключение к бд, иначе ответ в handler будет пуст
package api

import (
	"context"
	"errors"
	"log"
	"main/domain"
	"main/services/parser"
	"main/services/repo/redisdb"
	"main/services/req"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Стандартный курс для GetAll
const defaultSource = "RU"

// Логгер для API
var logger = log.New(os.Stdout, "API ", log.LstdFlags|log.Lshortfile)

// Сервис бд
type DatabaseService interface {
	// Получение данных по источнику и коду валюты. Возвращает сущность валюты пакета domain
	// и ненулевую ошибку при отключении от бд
	GetBySourceAndKey(ctx context.Context, source string, key string) (res domain.ValsDTO, err error)
	// Получение данных по источнику. Возвращает сущности валюты пакета domain
	// и ненулевую ошибку при отключении от бд
	GetAllBySource(ctx context.Context, source string) (res []domain.ValsDTO, err error)
	// Запись приведенных данных к сущности пакета domain.Возврашает ненулевую ошибку при отключении от бд
	Store(ctx context.Context, curr domain.ValsDTO) (err error)
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

type GetReqService interface {
	// GetCurrfromSource отправляет GET-запрос по ссылке для конкретного источника и конкретной валюты (если нужно, добавить ключи доступа).
	// Возвращает ненулевую ошибку при получении статуса запроса не OK
	GetCurrfromSource(source string, curr string, source_keys map[string]string, source_links map[string]string) (body []byte, err error)
}

// Хендлер запросов по ссылкам источника
type GetReqHandler struct {
	Service GetReqService
}

// Создание хендлера запросов по ссылкам источника. Нужна реализация интерфейса GetReqService
func NewGetReqHandler(svc GetReqService) *GetReqHandler {
	return &GetReqHandler{svc}
}

// API реализует запросы сервера, содержит данные о источниках, локации времени для сверки обновлений,
// и контекст для graceful shutdown
type API struct {
	//Клиент базы данных. При другой базе сменить клиент и пакет
	client       *redis.Client
	source_keys  map[string]string
	source_links map[string]string
	timeout      int
	time_loc     *time.Location
	gCtx         context.Context
}

// API реализует запросы сервера, содержит данные о источниках, локации времени для сверки обновлений,
// и контекст для graceful shutdown
func NewAPI(client *redis.Client, source_keys map[string]string, source_links map[string]string, timeout int, time_loc *time.Location, gCtx context.Context) *API {
	return &API{client, source_keys, source_links, timeout, time_loc, gCtx}
}

// Инициализация сервисов DatabaseService , ParseService
// Возвращает ошибку, если источник неверен
func (a *API) initServices(source string) (DatabaseHandler *DatabaseHandler, ParseHandler *ParseHandler, err error) {
	//Для каждого источника свой формат ответа
	var datatype string
	switch source {
	case "RU":
		datatype = "XML"
	case "TH":
		datatype = "JSON"
	default:
		return nil, nil, errors.New("wrong source provided")
	}
	DatabaseHandler = NewDatabaseHandler(redisdb.NewValsDTORepository(a.client))
	ParseHandler = NewParseHandler(parser.NewParser(datatype))

	return DatabaseHandler, ParseHandler, nil
}

// Обновление уже существующих валют.
// Возвращает ошибку если есть проблемы с подключением к БД или запрос к источнику вернул статус не OK
func (a *API) UpdateAllInSource(source string, time_loc *time.Location, time_to_update time.Time) (err error) {
	//Поиск данных, нахождение ненулевой даты последнего обновления, парсинг и запись в бд

	defaultMessage := "When updating all currencys error occured in method %s. Error: %s"
	DatabaseHandler, _, err := a.initServices(source)
	if err != nil {
		return err
	}

	//Поиск данных
	var DBDTO []domain.ValsDTO
	DBDTO, err = DatabaseHandler.Service.GetAllBySource(a.gCtx, source)
	if err != nil {
		logger.Printf(defaultMessage, "DatabaseService.GetAllBySource", err.Error())
		return err
	}
	//Поиск ненулевой даты для сверки на обновление ( на случай ошибки в бд)
	//Если не нашли, то берем предыдущий день
	var curr_time time.Time
	for _, v := range DBDTO {
		if len(v.Date) != 0 {
			curr_time, err = time.Parse(time.DateOnly, v.Date)
			if err != nil {

				return err
			}
			break
		}
	}

	if curr_time.Year() == 0 {
		curr_time = time.Now().In(time_loc).AddDate(0, 0, -1)
	}
	//Обновление всех данных
	for _, v := range DBDTO {
		err = a.NewOrUpdateCurr(source, v.Code, curr_time)
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
func (a *API) NewOrUpdateCurr(source string, curr string, currency_date time.Time) (err error) {
	//Cначала идет инициализация запросов,
	//затем получение информации из источника, затем парсинг и запись в бд данных

	//Инициализация сервисов
	DatabaseHandler, ParseHandler, err := a.initServices(source)
	if err != nil {
		return err
	}
	//Сервис отправки запросов
	GetReqHandler := NewGetReqHandler(req.NewGetReq(currency_date, a.time_loc, a.timeout))
	//Получение тела ответа
	body, err := GetReqHandler.Service.GetCurrfromSource(source, curr, a.source_keys, a.source_links)
	if err != nil {
		return err
	}
	//Парсинг тела ответа
	new_curr, err := ParseHandler.Service.Parse(source, body)
	if err != nil {
		return err
	}
	//При разных источниках разные структуры ответа
	switch source {
	case "RU":
		{
			RUDTO := new_curr.(*domain.RUsourceDTO)
			splt := strings.Split(RUDTO.Date, ".")
			//Приведение даты к формату yyyy-mm-dd
			new_date := splt[2] + "-" + splt[1] + "-" + splt[0]
			//Случай получения пустых данных
			if len(new_date) == 0 {
				return errors.New("parsed nil data from source RU for curr " + curr + ". abort")
			}
			for _, curr := range RUDTO.Valute {
				DatabaseHandler.Service.Store(a.gCtx, domain.ToValsDTO(
					new_date, "RU",
					curr.CharCode,
					curr.Name,
					curr.VunitRate, curr.VunitRate))
			}
			return nil

		}
	case "TH":
		{

			THDTO := new_curr.(domain.THsourceDTODataDetail)
			rg := regexp.MustCompile("[0-9]+")
			rg_s := rg.FindAllString(THDTO.Currency_Name_Eng, -1)
			init_rate_buy, err := strconv.ParseFloat(strings.Replace(THDTO.Buying_Transfer, ",", ".", 1), 64)
			if err != nil {
				return errors.New("wrong RatioBuy in source TH for curr " + curr + ". Abort")
			}
			init_rate_sell, err := strconv.ParseFloat(strings.Replace(THDTO.Selling, ",", ".", 1), 64)
			if err != nil {
				return errors.New("wrong RatioSell in source TH " + curr + ". Abort")
			}
			//Данные в этом источнике могут иметь отношение на определенный номинал. Далее идет нормализация
			//(соотношение 1 бата к единице искомой валюты)
			if len(rg_s) != 0 {
				amount, _ := strconv.ParseInt(rg_s[0], 10, 16)
				THDTO.Buying_Transfer = strconv.FormatFloat(init_rate_buy/float64(amount), 'f', 5, 64)
				THDTO.Selling = strconv.FormatFloat(init_rate_sell/float64(amount), 'f', 5, 64)
			}
			//Случай получения пустых данных
			if len(THDTO.Period) == 0 {
				return errors.New("parsed nil data from source TH. abort")
			}
			DatabaseHandler.Service.Store(a.gCtx,
				domain.ToValsDTO(THDTO.Period, "TH",
					THDTO.CurrencyID, THDTO.Currency_Name_Eng,
					THDTO.Buying_Transfer, THDTO.Selling))
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
	var course_types = []string{"buy", "sell"}
	for v := range a.source_keys {
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
	for _, v := range course_types {
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
func (a *API) checkNameFromSource(source string, name string, db *DatabaseHandler, ctx context.Context) (nameDTO domain.ValsDTO, err error) {
	//Проверка на курс источника
	if strings.Contains(name, source) {
		date := time.Now().Format(time.DateOnly)
		nameDTO = domain.ToValsDTO(date, source, name, name, "1.0", "1.0")
	}
	//Поиск записи
	nameDTO, err = db.Service.GetBySourceAndKey(ctx, source, name)
	if err != nil {
		return domain.ValsDTO{}, err
	}
	//Если нет
	if len(nameDTO.Name) == 0 {
		//Добавление новой валюты
		err = a.NewOrUpdateCurr(source, name, time.Now())
		if err != nil {
			logger.Printf("Cannot find data for this curr %s. Error: %e", name, err)
			return domain.ValsDTO{}, err
		}
	}
	return nameDTO, nil
}

// Метод Сonvert реализует запрос '/convert'. Достает из бд данные о валютах источника, и
// при выбранном курсе перевода ( продажа/покупка), выводит тело ответа с данными о валютах и переведенном номинале
// Возвращает ошибку если неправильно введены параметры или проблема с бд
func (a *API) Convert(source string, first string, second string, amount string, course string) (data interface{}, err error) {
	defaultMessage := "In Convert error occured in method %s. Error: %s"
	defaulterr := errors.New("cannot provide now convert. try again later")
	//Проверка на правильность ввода
	err = a.checkQuery(source, first, second, amount, course)
	if err != nil {
		return nil, err
	}
	DatabaseHandler, _, err := a.initServices(source)
	if err != nil {
		logger.Printf(defaultMessage, "API.initServices", err.Error())
		return nil, defaulterr
	}
	firstDTO, err := a.checkNameFromSource(source, first, DatabaseHandler, a.gCtx)
	if err != nil {
		logger.Printf(defaultMessage, "API.checkNameFromSource", err.Error())
		return nil, defaulterr
	}
	secondDTO, err := a.checkNameFromSource(source, second, DatabaseHandler, a.gCtx)
	if err != nil {
		logger.Printf(defaultMessage, "API.checkNameFromSource", err.Error())
		return nil, defaulterr
	}
	amount_parsed, err := strconv.ParseFloat(strings.Replace(amount, ",", ".", 1), 64)
	if err != nil {
		return nil, errors.New("wrong amount passed")
	}
	var firstRatio, secondRatio float64
	//Обработка выбора курса продажи или покупки
	switch course {
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
	converted_amount := amount_parsed * firstRatio / secondRatio
	type DataforConvert struct {
		Date             string `json:"date,omitempty"`
		Source           string `json:"source,omitempty"`
		First            string `json:"first_curr,omitempty"`
		Second           string `json:"second_curr,omitempty"`
		Course           string `json:"course,omitempty"`
		Amount           string `json:"amount,omitempty"`
		Converted_amount string `json:"converted_amount,omitempty"`
	}
	var res DataforConvert
	/*Приведение к виду ответа с данными*/
	res.Date = firstDTO.Date
	res.Source = firstDTO.Source
	res.First = firstDTO.Code
	res.Second = secondDTO.Code
	res.Course = course
	res.Amount = amount
	res.Converted_amount = strconv.FormatFloat(converted_amount, 'f', 3, 64)

	return res, nil
}

// Метод  реализует запрос '/getAll'. Достает из бд данные о валютах источника. Возвращает ошибку если потеряно соединене с бд после 5 попыток
func (a *API) GetAll(source string) (ans []domain.ValsDTO, err error) {

	if len(source) == 0 {
		source = defaultSource
	}
	DatabaseHandler, _, _ := a.initServices(source)
	sourceDTOs, err := DatabaseHandler.Service.GetAllBySource(a.gCtx, source)
	if err != nil {
		logger.Printf("Check logs for DB. Error:%e", err)
		return nil, errors.New("when requesting  data from database error occured. Try again later")
	}
	return sourceDTOs, nil
}
