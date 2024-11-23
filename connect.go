package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

/*Обработка на возможный таймаут или недоступность источника*/
func switchResponse(code int, curr string, source string, err error, name string) error {
	if code == http.StatusBadRequest {
		err = errors.New("Currency " + curr +
			" is not presented in source " + source + " or currency's name is invalid")
	} else if code == http.StatusRequestTimeout {
		err = errors.New("Timeout. Try again later")
	} else {
		log.Printf(name+", %s, %s \n", err.Error(), time.Now().In(loc))
	}
	return err
}

func DbConn(w http.ResponseWriter, r *http.Request) (*redis.Client, int, error) {
	opt, _ := redis.ParseURL(redis_db)
	client := redis.NewClient(opt)
	err := client.Conn().Ping(ctx).Err()
	/* Проверка работы базы данных*/
	if err != nil {
		log.Printf("DbConn, %s, %s \n", err.Error(), time.Now().In(loc))
		return client, http.StatusInternalServerError, http.ErrServerClosed
	}
	return client, http.StatusOK, nil
}

func GetonLink(link string, sourceType string, w http.ResponseWriter, r *http.Request) (*http.Response, int, error) {
	client := &http.Client{
		Timeout: timeout,
	}
	req, _ := http.NewRequest("GET", link, nil)
	/* Пока что написаны парсеры для таких источников данных*/
	req.Header.Add("Accept", `application/xml; application/json`)
	/* Обход защиты от автоматизированных запросов*/
	req.Header.Add("User-Agent", `Chrome/23.0.1271.64`)
	if strings.Compare(sourceType, "jsonTH") == 0 {
		req.Header.Add("X-IBM-Client-Id", source_keys["TH"])
		body := r.URL.RawQuery
		requrl, err := url.ParseQuery(body)
		if err != nil {
			return &http.Response{}, http.StatusBadRequest, errors.New("Wrong query with " + link + " passed to parse")
		}
		/* Приведение параметров к нужному виду*/
		req.URL.RawQuery = "start_period=" + requrl.Get("start_period") +
			"&end_period=" + requrl.Get("end_period") + "&currency=" + requrl.Get("currency")

	}

	res, err := client.Do(req)
	if err != nil {
		/* Сервис недоступен или не прошел таймаут*/
		log.Printf("GetonLink, %s, %s \n \n", err, time.Now().In(loc))
		return &http.Response{}, http.StatusRequestTimeout, err
	}
	if res.StatusCode > 299 {
		/* Обработка статус кода*/
		body, _ := io.ReadAll(res.Body)
		var RespErr map[string][]interface{}
		var ErrBody string
		_ = json.NewDecoder(bytes.NewReader(body)).Decode(&RespErr)
		if strings.Compare(sourceType, "jsonTH") == 0 {
			/* Для источника ЦБ Тайланд*/
			ErrBody = RespErr["moreInformation"][0].(map[string]interface{})["message"].(string)
		}
		log.Printf("GetonLink, %s, %s \n \n", errors.New("In source "+link+" error occured. Body:"+ErrBody), time.Now().In(loc))
		return res, res.StatusCode, errors.New(ErrBody)
	}
	return res, http.StatusOK, nil
}
