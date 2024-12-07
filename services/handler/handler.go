// handler реализует запросы клиента и посылает запрос в бизнес-логику и получает ответ
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"main/domain"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Сервис API
type APIservice interface {
	//Реализация запроса '/convert'
	Convert(source string, first string, second string, amount string, course string) (data interface{}, err error)
	//Реализация запроса '/getAll'
	GetAll(source string) (ans []domain.ValsDTO, err error)
	//Реализация горутины для периодического обновления данных
	UpdateAllInSource(source string, time_loc *time.Location, time_to_update time.Time) (err error)
}

// Хендлер API
type APIHandler struct {
	Service APIservice
}

// Handler реализует запросы клиента и посылает запрос в бизнес-логику и получает ответ
func NewAPIHandler(svc APIservice) *APIHandler {
	return &APIHandler{svc}
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
func (r Response) Response(w http.ResponseWriter) {
	resp, err := json.Marshal(r)
	if err != nil {
		log.Printf("Cannot write response with %s and message %s in %s", strconv.FormatInt(int64(r.Code), 10), r.Message, w)
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

// '/convert'
func (ah *APIHandler) Convert(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", `application/json`)
	var resp Response
	params, err := url.ParseQuery(r.URL.RawQuery)
	/*Проверка на правильность ввода параметров */
	if err != nil || len(params) == 0 {
		resp.SetAnswer(http.StatusBadRequest, "Wrong query passed", nil)
		resp.Response(w)
		return
	}
	data, err := ah.Service.Convert(params.Get("source"), params.Get("first"),
		params.Get("second"), params.Get("amount"), params.Get("course"))
	if err != nil {
		resp.SetAnswer(http.StatusBadRequest, err.Error(), nil)
		resp.Response(w)
		return
	}
	resp.SetAnswer(0, "", append(resp.Data, data))
	resp.Response(w)

}

// '/getAll'
func (ah *APIHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	var resp Response
	w.Header().Set("Content-Type", `application/json`)
	params, err := url.ParseQuery(r.URL.RawQuery)
	/*Проверка на правильность ввода параметров */
	if err != nil || len(params) == 0 {
		resp.SetAnswer(http.StatusBadRequest, "Wrong query passed", []interface{}{})
		resp.Response(w)
		return
	}
	data, err := ah.Service.GetAll(params.Get("source"))
	if err != nil {
		resp.SetAnswer(http.StatusInternalServerError, err.Error(), []interface{}{})
		resp.Response(w)
		log.Printf("GetAll: " + err.Error())
	}
	resp.SetAnswer(0, "", append(resp.Data, data))
	resp.Response(w)

}
func (ah *APIHandler) Greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Convertation service. Use `/convert`. %s", time.Now())
}

// Периодическое обновление данных
func (ah *APIHandler) Update(time_wait int, loc_time *time.Location, time_in_day map[string]time.Time, sources []string, ctx context.Context) (err error) {
	//Флаг на обновленность( чтобы каждый раз запросы не делались на сервера источника)
	updated_sources := make([]bool, len(time_in_day))
	//Время последнего обновления (отслежка нового дня)
	lastupdate := time.Now().In(loc_time)
	//Логгер отслеживания обновления
	logger := log.New(os.Stdout, "Update ", log.Lshortfile)
	for {
		select {
		case <-time.After(time.Duration(time_wait) * time.Second):
			for k, i := range sources {
				select {
				case <-ctx.Done():
					err = errors.New("exiting service")
					return err
				default:
					t := time.Now().In(loc_time)
					//Eсли настал новый день
					if t.Day()-lastupdate.Day() > 0 {
						updated_sources[k] = false
					}
					//Приведение даты к нужноу виду
					time_in_day[i] = time_in_day[i].AddDate(t.Year(), int(t.Month())-1, t.Day()-1)
					time_for_update := time_in_day[i]
					isWeekend := (int(time_for_update.Weekday()) == 6) || (int(time_for_update.Weekday()) == 0)
					//При запуске обновляется, потом проверка на время обновления
					if t.Before(time_for_update) || (updated_sources[k] && t.After(time_for_update)) || isWeekend {
						//Не настало время обновления
						logger.Printf("Currently no updates available for source " + i)
					} else if !updated_sources[k] {
						//Обновление
						logger.Printf("Starting to update data for source " + i)
						err := ah.Service.UpdateAllInSource(i, loc_time, time_for_update)
						if err != nil {
							logger.Printf("Cannot update in source " + i + ". Will try again later Error:" + err.Error())
							updated_sources[k] = false
						} else {
							logger.Printf("Succsessfully updated data for source " + i)
							updated_sources[k] = true
						}
					}
					lastupdate = time.Now().In(loc_time)
				}
			}
		case <-ctx.Done():
			//Graceful shutdown
			err = errors.New("exiting")
			return err
		}
	}

}
