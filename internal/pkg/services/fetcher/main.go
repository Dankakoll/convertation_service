// Пакет fetcher предоставляет доступ к внешним источникам данных
// обрабатывает запросы по ссылкам
package fetcher

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

const SourceRU = "RU"
const SourceTH = "TH"

// Реализует Fetcher
type Fetcher struct {
	//Время последнего обновления
	lastUpdate time.Time
	//Локация
	timeLoc *time.Location
	timeout int
}

// Cоздание Fetcher. Предоставляет доступ к внешним источникам данных
// обрабатывает запросы по ссылкам. Требуется последнее время обновления, локация времени, и timeout
func NewFetcher(lastUpdate time.Time, timeLoc *time.Location, timeout int) *Fetcher {
	return &Fetcher{lastUpdate, timeLoc, timeout}
}

// Добавление нужных заголовков к запросу в зависимости от источника
func (f *Fetcher) reqBySource(source string, curr string, sourceKeys map[string]string, sourceLinks map[string]string) (*http.Request, error) {
	t := time.Now().In(f.timeLoc)
	var ctx = context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", sourceLinks[source], nil)
	if err != nil {
		return nil, err
	}
	switch source {
	case SourceRU:
		{
			req.Header.Add("Accept", `application/xml`)
			req.Header.Add("User-Agent", `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.64 Safari/537.11`)
		}
	case SourceTH:
		{
			req.Header.Add("Accept", `application/json`)
			if sourceKeys["TH"] == "" {
				logger := log.New(os.Stdout, "Fetcher ", log.LstdFlags|log.Lshortfile)
				logger.Print("Cannot fetch source TH. No key provided")
				return nil, errors.New("no key provided for source th")
			}
			req.Header.Add("X-IBM-Client-Id", sourceKeys["TH"])
			lastup := f.lastUpdate
			startPeriod := lastup.Format(time.DateOnly)
			endPeriod := t.Format(time.DateOnly)
			req.URL.RawQuery = "start_period=" + startPeriod +
				"&end_period=" + endPeriod + "&currency=" + curr
		}
	}
	return req, nil
}

// Вывод тела ошибки в ответ АПИ
func (f *Fetcher) errBodyBySource(source string, body []byte) string {
	var ErrBody string
	switch source {
	/* Для источника ЦБ Тайланд*/
	case SourceTH:
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
	return ErrBody
}

// GetCurrfromSource отправляет GET-запрос по ссылке для конкретного источника и конкретной валюты (если нужно, добавить ключи доступа).
// Возвращает ненулевую ошибку при получении статуса запроса не OK
func (f *Fetcher) GetCurrfromSource(source string, curr string, sourceKeys map[string]string, sourceLinks map[string]string) (body []byte, err error) {
	//Логгер для requester
	logger := log.New(os.Stdout, "requester", log.LstdFlags|log.Lshortfile)
	//Проверка на время обновления данных
	t := time.Now().In(f.timeLoc)
	timeout := time.Duration(f.timeout) * time.Second
	httpClient := &http.Client{Timeout: timeout}
	req, err := f.reqBySource(source, curr, sourceKeys, sourceLinks)
	if err != nil {
		return body, err
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
		ErrBody := f.errBodyBySource(source, body)
		logger.Printf("Request to source %s failed, returned this error message: %s", source, ErrBody)
		return body, errors.New(ErrBody)
	}
	f.lastUpdate = t
	return body, nil
}
