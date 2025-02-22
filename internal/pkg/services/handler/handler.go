// handler реализует запросы клиента и посылает запрос в бизнес-логику и получает ответ
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"main/config"
	"main/internal/pkg/domain"
	"main/internal/pkg/services/api"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Логгер для Handler
var logger = log.New(os.Stdout, "Handler ", log.LstdFlags|log.Lshortfile)

// Сервис API
type APIservice interface {
	//Реализация запроса '/convert'
	Convert(source string, first string, second string, amount string, course string) (data interface{}, err error)

	//Реализация запроса '/getAll'
	GetAll(source string) (ans []domain.CurrModel, err error)

	//Реализация горутины для периодического обновления данных
	UpdateAllInSource(source string, timeLoc *time.Location, timeToUpdate time.Time) (err error)

	//Реализация отключения от бд
	ExitConnectWithDb(mainCtx context.Context) (err error)

	//Получение неактуальной записи
	GetDateFromSource(source string) (init_date time.Time, err error)
}

// Хендлер API
type APIHandler struct {
	Service APIservice
}

// Handler реализует запросы клиента и посылает запрос в бизнес-логику и получает ответ
func NewAPIHandler(svc APIservice, err error) (*APIHandler, error) {
	return &APIHandler{svc}, err
}

// Стурктура ответа
type Response struct {
	//Код ответа
	Code int `json:"code,omitempty"`
	//Сообщение для пользователя
	Message string `json:"message,omitempty"`
	//Данные
	Data []interface{} `json:"data,omitempty"`
}

// Вывод ответа в клиент
func (r Response) WriteResp(w http.ResponseWriter) {
	resp, err := json.Marshal(r)
	if err != nil {
		logger.Printf("Cannot write response with %s and message %s in %s", strconv.FormatInt(int64(r.Code), 10), r.Message, w)
		return
	}
	fmt.Fprintf(w, "%s", resp)
}

// Ввод параметров ответа для структуры Response
func (r *Response) SetAnswer(code int, message string, data []interface{}) {
	r.Code = code
	r.Message = message
	if data != nil {
		r.Data = data
	}
}

// Инициализация роутов для хендлера
func InitHandler(AppConfig *config.AppConfig, mainCtx context.Context) (ah *APIHandler, err error) {
	ah, err = NewAPIHandler(api.NewAPI(AppConfig.DbUrl, AppConfig.SourceKeys, AppConfig.SourceLinks, AppConfig.TimeoutREQ, AppConfig.Loc, mainCtx, AppConfig.DbAttempts))
	if err != nil {
		return ah, err
	}
	http.HandleFunc("/", ah.greet)
	http.HandleFunc("/getall", ah.getAll)
	http.HandleFunc("/convert", ah.convert)
	return ah, nil
}

// Запуск горутины обновления данных
func (ah *APIHandler) StartUpdate(AppConfig *config.AppConfig, mainCtx context.Context) error {
	var err error
	//Время обновления
	var timeUpd = make(map[string]time.Time, len(AppConfig.Sources))
	for _, v := range AppConfig.Sources {
		timeUpd[v], err = AppConfig.ParseTime(v)
		if err != nil {
			return err
		}
	}

	return ah.UpdatingSources(AppConfig.TimeoutUP, AppConfig.Loc, timeUpd, AppConfig.Sources, mainCtx)
}

// Горутина для отслеживания соединения с бд
func (ah *APIHandler) ExitConnectWithDb(mainCtx context.Context) error {
	return ah.Service.ExitConnectWithDb(mainCtx)
}

// Convert godoc
// @Summary 	Конвертация валют
// @Description Конвертация валют в зависимости от источника, требуется предоставление двух кодов валют, суммы конвертации, и курса обмена.
// @Tags 		handlerConvert
// @ID 			Convert
// @Param 		source 		path 	string 		true 	"source"
// @Param 		first 		path 	string 		true 	"first"
// @Param 		second 		path 	string 		true 	"second"
// @Param 		amount 		path 	string 		true 	"amount"
// @Param 		exchange 	path 	string 		true 	"exchange"
// @Success 	200 	  {object} 	handler.Response{data=api.ConvertResponse}
// @Failure 	400 	  {object}  handler.Response
// @Failure 	404 	  {object}  handler.Response
// @Failure		500 	  {object} 	handler.Response
// @Router 		/convert   [get]
// @Examples     /convert?source=TH&first=RUB&second=USD&amount=1000&exchange=buy
func (ah *APIHandler) convert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", `application/json`)
	var resp Response
	params, err := url.ParseQuery(r.URL.RawQuery)
	/*Проверка на правильность ввода параметров */
	if err != nil || len(params) == 0 {
		resp.SetAnswer(http.StatusBadRequest, "Wrong query passed", nil)
		resp.WriteResp(w)
		return
	}
	data, err := ah.Service.Convert(params.Get("source"), params.Get("first"),
		params.Get("second"), params.Get("amount"), params.Get("exchange"))
	if err != nil {
		resp.SetAnswer(http.StatusBadRequest, err.Error(), nil)
		resp.WriteResp(w)
		return
	}
	resp.SetAnswer(http.StatusOK, "Conversion successful", []interface{}{data})
	resp.WriteResp(w)

}

// GetAll godoc
// @Summary		 Получить все валюты
// @Description	 Получить все валюты из источника. Если источник не указан, берутся данные из источника по умолчанию (ЦБ РФ)
// @Tags 	 	 GetAll
// @ID 			 getAll
// @Accept 		 json
// @Produce  	 json
// @Param 		 source 	path 		string 		true 	"source"
// @Success 	 200 	  {object} 		handler.Response{data=[]domain.CurrModel}
// @Failure 	 400 	  {object}  handler.Response
// @Failure 	 404 	  {object}  handler.Response
// @Failure		 500 	  {object} 	handler.Response
// @Router 		 /getAll		 		[get]
// @Examples      /getAll?source=RU
func (ah *APIHandler) getAll(w http.ResponseWriter, r *http.Request) {
	var resp Response
	w.Header().Set("Content-Type", `application/json`)
	params, err := url.ParseQuery(r.URL.RawQuery)
	//Проверка на правильность ввода параметров
	if err != nil || len(params) == 0 {
		resp.SetAnswer(http.StatusBadRequest, "Wrong query passed", []interface{}{})
		resp.WriteResp(w)
		return
	}
	data, err := ah.Service.GetAll(params.Get("source"))
	if err != nil {
		resp.SetAnswer(http.StatusBadRequest, err.Error(), []interface{}{})
		resp.WriteResp(w)
		logger.Printf("%s", "GetAll: "+err.Error())
		return
	}
	resp.SetAnswer(http.StatusOK, "Getting all queries from source "+params.Get("source")+" successful", []interface{}{data})
	resp.WriteResp(w)

}
func (ah *APIHandler) greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Convertation service. Use `/convert`. %s", time.Now())
}

// Периодическое обновление данных
func (ah *APIHandler) update(locTime *time.Location, timeInDay map[string]time.Time, sources []string, ctx context.Context) (err error) {
	defaultMessage := "Update: "
	select {
	case <-ctx.Done():
		logger.Print(defaultMessage + "Shutdowning gorotine")
		return errors.New("exiting update")
	default:
		//Флаг на обновленность( чтобы каждый раз запросы не делались на сервера источника)
		updatedSources := make([]bool, len(timeInDay))
		for k, i := range sources {
			t := time.Now().In(locTime)
			//Поиск последней даты
			init_date, err := ah.Service.GetDateFromSource(i)
			logger.Printf("last date %s", init_date.Format(time.DateOnly))
			if err != nil {
				logger.Printf("%s", defaultMessage+"Cannot update in source "+i+". Will try again later Error:"+err.Error())
				updatedSources[k] = false
			}
			//Eсли настал новый день
			if t.Day()-init_date.Day() > 0 {
				updatedSources[k] = false
			}
			//Приведение даты к нужноу виду
			timeInDay[i] = timeInDay[i].AddDate(init_date.Year(), int(init_date.Month())-1, init_date.Day()-1)
			timeForUpdate := timeInDay[i]
			isWeekend := (int(timeForUpdate.Weekday()) == 6) || (int(timeForUpdate.Weekday()) == 0)
			//При запуске обновляется, потом проверка на время обновления
			if t.Before(timeForUpdate) || (updatedSources[k] && t.After(timeForUpdate)) || isWeekend || err != nil {
				//Не настало время обновления или произошла ошибка при получении данных
				logger.Printf("%s", defaultMessage+"Currently no updates available for source "+i)
			} else if !updatedSources[k] {
				//Обновление
				logger.Printf("%s", defaultMessage+"Starting to update data for source "+i)
				err := ah.Service.UpdateAllInSource(i, locTime, timeForUpdate)
				if err != nil {
					logger.Printf("%s", defaultMessage+"Cannot update in source "+i+". Will try again later Error:"+err.Error())
					updatedSources[k] = false
				} else {
					logger.Printf("%s", defaultMessage+"Succsessfully updated data for source "+i)
					updatedSources[k] = true
				}
			}
		}
	}
	return nil

}

// Запуск update с получением сигнала о остановке работы
func (ah *APIHandler) UpdatingSources(timeWait int, locTime *time.Location, timeInDay map[string]time.Time, sources []string, ctx context.Context) (err error) {
	logger.Printf("%s", "Starting update")
	//Канал для первичного обновления
	first_launch := make(chan bool, 1)
	first_launch <- true
	for {
		select {
		case <-first_launch:
			err = ah.update(locTime, timeInDay, sources, ctx)
			if err != nil {
				return err
			}
		case <-time.After(time.Duration(timeWait) * time.Second):
			err = ah.update(locTime, timeInDay, sources, ctx)
			if err != nil {
				return err
			}
		//На случай завершения обновления и получения сигнала SIGTERM. Бизнес-логика завершится и
		//горутина закроется без гонки
		case <-ctx.Done():
			//Graceful shutdown
			logger.Print("Gracefully stopping Update")
			return errors.New("update stopped")

		}
	}
}
