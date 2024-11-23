package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

func XMLparser(link string, w http.ResponseWriter, r *http.Request) (any, int, error) {
	/*Get запрос в источник данных*/
	res, status, err := GetonLink(link, "xml", w, r)
	if err != nil {
		err = switchResponse(res.StatusCode, r.URL.Query().Get("currency"), r.URL.Query().Get("source"), err, "XMLparser")
		return new(any), status, err
	}
	d := xml.NewDecoder(res.Body)
	d.CharsetReader = charset.NewReaderLabel
	/*Приведение в нужную кодировку*/
	var source any
	if strings.Contains(link, "cbr.ru") {
		/*В зависимости от источника приводим объект к нужному типу*/
		source = new(CBRVals)
	}
	err = d.Decode(&source)
	if err != nil { /*Если не xml*/
		log.Printf("XMLparser, %s, %s \n", http.ErrBodyNotAllowed.Error(), time.Now().In(loc))
		return new(any), http.StatusMethodNotAllowed, http.ErrBodyNotAllowed
	}
	defer res.Body.Close()
	return source, http.StatusOK, nil
}
func JSONparser(link string, w http.ResponseWriter, r *http.Request) (any, int, error) {
	ResponseType := "json"
	var new_source any
	var source any
	if strings.Contains(link, "apigw1.bot.or.th") {
		ResponseType += "TH"
		var rangetime time.Time
		/*Введение нужных параметров запроса, в зависимости от текущего времени
		результат запроса может быть пустым если базы не обновились в */
		flag := int(t.Weekday()) == 6 || int(t.Weekday()) == 7
		if time_before {
			rangetime = t.AddDate(0, 0, -1)
		} else if !flag {
			rangetime = t
		} else {
			rangetime = t.AddDate(0, 0, -int(t.Weekday()-time.Friday))
		}
		start_period := rangetime.Format(time.DateOnly)
		end_period := t.Format(time.DateOnly)
		new_query := r.URL.Query()
		new_query.Add("start_period", start_period)
		new_query.Add("end_period", end_period)
		r.URL.RawQuery = new_query.Encode()
		/*Для источника ЦБ Тайланд*/
		source = new(THVals)
		new_source = new(THValsDataDetail)
	}
	res, status, err := GetonLink(link, ResponseType, w, r)

	if err != nil {
		err = switchResponse(res.StatusCode, r.URL.Query().Get("currency"), r.URL.Query().Get("source"), err, "JSONparser")
		return new(any), status, err
	}
	err = json.NewDecoder(res.Body).Decode(&source)
	if err != nil {
		log.Printf("JSONparser, %s, %s \n", "wrong body to parse JSON", time.Now().In(loc))
		return new(any), http.StatusNotAcceptable, http.ErrBodyNotAllowed
	}
	defer res.Body.Close()
	if strings.Contains(link, "apigw1.bot.or.th") {
		/* Данный источник имеет вложенную структуру,
		требуется дополнительное объявление еще  одного объекта */
		DataDetail, _ := json.Marshal(source.(*THVals).Result.Data.DataDetail[0])
		err = json.NewDecoder(bytes.NewReader(DataDetail)).Decode(&new_source)
		if err != nil {
			log.Printf("JSONparser, %s, %s \n", errors.New("nil internal body to parse"), time.Now().In(loc))
			return new(any), http.StatusNotAcceptable, http.ErrBodyNotAllowed
		}
		return new_source, http.StatusOK, nil
	} else {
		return source, http.StatusOK, nil
	}
}
