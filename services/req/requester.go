// Пакет req предоставляет доступ к внешним источникам данных
// обрабатывает запросы по ссылкам
package req

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Реализует Requester
type GetReq struct {
	//Время последнего обновления
	lastupdate time.Time
	//Локация
	time_loc *time.Location
	timeout  int
}

// Cоздание Requester. Предоставляет доступ к внешним источникам данных
// обрабатывает запросы по ссылкам. Требуется последнее время обновления, локация времени, и timeout
func NewGetReq(lastupdate time.Time, time_loc *time.Location, timeout int) *GetReq {
	return &GetReq{lastupdate, time_loc, timeout}
}

// GetCurrfromSource отправляет GET-запрос по ссылке для конкретного источника и конкретной валюты (если нужно, добавить ключи доступа).
// Возвращает ненулевую ошибку при получении статуса запроса не OK
func (g *GetReq) GetCurrfromSource(source string, curr string, source_keys map[string]string, source_links map[string]string) (body []byte, err error) {
	//Логгер для requester
	logger := log.New(os.Stdout, "requester", log.LstdFlags|log.Lshortfile)
	//Проверка на время обновления данных
	t := time.Now().In(g.time_loc)
	timeout := time.Duration(g.timeout) * time.Second
	httpClient := &http.Client{Timeout: timeout}
	var ctx = context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", source_links[source], nil)
	if err != nil {
		return body, err
	}
	switch source {
	case "RU":
		{
			req.Header.Add("Accept", `application/xml`)
			req.Header.Add("User-Agent", `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.64 Safari/537.11`)
		}
	case "TH":
		{
			req.Header.Add("Accept", `application/json`)
			req.Header.Add("X-IBM-Client-Id", source_keys["TH"])
			lastup := g.lastupdate
			start_period := lastup.Format(time.DateOnly)
			end_period := t.Format(time.DateOnly)
			req.URL.RawQuery = "start_period=" + start_period +
				"&end_period=" + end_period + "&currency=" + curr
		}
	}
	res, err := httpClient.Do(req)
	if err != nil {
		/* Сервис недоступен или не прошел таймаут*/
		logger.Printf("Source %s currently unavailable due to timeout", source)
		return body, err
	}
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode > 299 {
		/* Обработка статус кода*/
		var ErrBody string
		switch source {
		/* Для источника ЦБ Тайланд*/
		case "TH":
			{
				var RespErr map[string][]interface{}
				_ = json.NewDecoder(bytes.NewReader(body)).Decode(&RespErr)
				ErrBody = RespErr["moreInformation"][0].(map[string]interface{})["message"].(string)
			}
		default:
			{
				ErrBody = string(body)
			}
		}

		logger.Printf("Request for source %s returned this Error message: %s", source, ErrBody)
		return body, errors.New(ErrBody)
	}
	g.lastupdate = t
	return body, nil
}
