package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

/*Время (UTC+7)*/
var loc, _ = time.LoadLocation("Asia/Bangkok")
var t = time.Now().In(loc)
var def = `RU`
var source_keys = make(map[string]string, 1)
var source_currs = []string{"RUB", "THB"}
var ctx = context.Background()
var timeout = time.Duration(20 * time.Second)
var redis_db string

/*Время обновления БД Тайланд*/
var time_before = t.Before(time.Date(t.Year(), t.Month(), t.Day(), 18, 0, 0, 0, loc))

/*Валюта*/
type Vals struct {
	Date   string `redis:"Date" json:"date"`
	Source string `redis:"Source" json:"-"`
	Code   string `redis:"Code" json:"Code"`
	Name   string `redis:"Name" json:"Name"`
	Ratio  string `redis:"Ratio" json:"Ratio"`
}

/*Ответ от АПИ*/
type Response struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Data    []Data `json:"data,omitempty"`
}

/*Тело ответа для /update и /getall*/
type Data struct {
	/*Ответ /convert*/
	Date             string `json:"date,omitempty"`
	Source           string `json:"source,omitempty"`
	First            string `json:"first_curr,omitempty"`
	Second           string `json:"second_curr,omitempty"`
	Amount           string `json:"amount,omitempty"`
	Converted_amount string `json:"converted_amount,omitempty"`
	/*Ответ /getall*/
	Code  string `json:"code,omitempty"`
	Name  string `json:"name,omitempty"`
	Ratio string `json:"ratio,omitempty"`
}

type Config struct {
	Endpoint string
	Timeout  time.Duration
}

/*Вывод ответа*/
func (r Response) Response(w http.ResponseWriter) {
	resp, err := json.Marshal(r)
	if err != nil {
		log.Printf("Cannot write response with %s and message %s in %s", strconv.FormatInt(int64(r.Code), 10), r.Message, w)
		return
	}
	fmt.Fprintf(w, "%s", resp)
}
func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Convertation service. Use `/convert`. %s", time.Now().In(loc))
}

/*Проверка на актуальность введенных данных или их обновление*/
func checkNotNilorUptoDate(w http.ResponseWriter, r *http.Request, client *redis.Client, vals Vals, name string) (Vals, int, error) {
	t := time.Now().In(loc)
	time_before := t.Before(time.Date(t.Year(), t.Month(), t.Day(), 18, 0, 0, 0, loc))
	params, _ := url.ParseQuery(r.URL.RawQuery)
	key := params.Get("source")
	/*Если источник не указан*/
	if len(key) == 0 {
		key = def
	}
	curr := params.Get(name)
	err := client.HGetAll(ctx, key+":"+curr).Scan(&vals)
	if err != nil {
		/* Ошибка на случай неверного контекста или отключения бд после пройденной проверки,
		или занятость транзакции*/
		return vals, http.StatusNotFound, err
	}
	var uptoDate time.Time
	currDate, _ := time.Parse(time.DateOnly, t.Format(time.DateOnly))
	flag := int(currDate.Weekday()) == 6 || int(currDate.Weekday()) == 7
	if len(vals.Date) != 0 {
		uptoDate, err = time.Parse(time.DateOnly, vals.Date)
		/*Неправильная дата в базе*/
		if err != nil {
			log.Printf("checkNotNilorUptoDate, %s, %s \n", err.Error(), time.Now().In(loc))
			return vals, http.StatusNotAcceptable, http.ErrNotSupported
		}
	} else if flag {
		/*(проверка на выходной день
		-берем пятничные данные)*/
		uptoDate = currDate.AddDate(0, 0, -int(currDate.Weekday()-time.Friday))
	} else {
		uptoDate = currDate.AddDate(0, 0, -1)
	}
	if len(vals.Code) == 0 || ((!time_before) && uptoDate.Before(currDate)) {
		/*Если в базе нет данной валюты с источника
		или наступило время обновления данных. Ввиду ограничение на запросы в день
		было решено ввести такое ограничение, чтобы постоянно не подключаться к источнику*/
		var insourcecurr = false
		for _, el := range source_currs {
			if strings.Compare(el, curr) == 0 {
				insourcecurr = true
				break
			}
		}
		if strings.Contains(curr, key) && insourcecurr {
			/*Если страна валюты совпадает с источником, то используем соответствующую источнику
			валюту с отношением 1*/
			vals.Date = currDate.Format(time.DateOnly)
			vals.Code = params.Get(name)
			vals.Source = params.Get("source")
			vals.Ratio = "1,0"
		} else if len(curr) == 3 && regexp.MustCompile(`^[A-Za-z]+$`).MatchString(curr) {
			/* Проверка на правильный ввод ( код валюты состоит из 3 букв) */
			new_query, _ := url.ParseQuery("source=" + key + "&currency=" + params.Get(name))
			r.URL.RawQuery = new_query.Encode()
			status, err := update(w, r)
			if err != nil {
				log.Printf("checkNotNilorUptoDate, %s, %s \n", err.Error(), time.Now().In(loc))
				return vals, status, err
			}
			r.URL.RawQuery = params.Encode()
			_ = client.HGetAll(ctx, key+":"+params.Get(name)).Scan(&vals)
		} else if len(curr) > 0 {
			return vals, http.StatusBadGateway, errors.New(name + ": wrong currency name " + curr)
		} else {
			return vals, http.StatusBadGateway, errors.New(name + ": no currency provided")
		}
	}
	return vals, http.StatusOK, nil
}

func convertCurrs(w http.ResponseWriter, r *http.Request) {
	/*Вывод в формате JSON*/
	w.Header().Set("Content-Type", "application/json")
	var resp Response
	t = time.Now().In(loc)
	/*Время обновления источника ЦБ Тайланд (utc+7) */
	time_before = t.Before(time.Date(t.Year(), t.Month(), t.Day(), 18, 0, 0, 0, loc))
	client, status, err := DbConn(w, r)
	/*Проверка на доступность бд */
	if err != nil {
		log.Printf("convertCurrs, %s, %s \n", err.Error(), time.Now().In(loc))
		resp.Code = status
		resp.Message = err.Error()
		resp.Response(w)
		return
	}
	params, err := url.ParseQuery(r.URL.RawQuery)
	/*Проверка на правильность ввода параметров */
	if err != nil || len(params) == 0 {
		resp.Code = http.StatusBadGateway
		resp.Message = "Wrong query"
		resp.Response(w)
		log.Printf("Wrong query")
		return
	}
	var first, second Vals
	/*Получение данных валют */
	first, status, err = checkNotNilorUptoDate(w, r, client, first, "first")
	if err != nil {
		log.Printf("convertCurrs, %s, %s \n", err.Error(), time.Now().In(loc))
		resp.Code = status
		resp.Message = err.Error()
		resp.Response(w)
		return
	}
	second, status, err = checkNotNilorUptoDate(w, r, client, second, "second")
	if err != nil {
		log.Printf("convertCurrs, %s, %s \n", err.Error(), time.Now().In(loc))
		resp.Code = status
		resp.Message = err.Error()
		resp.Response(w)
		return
	}
	/*Приведение к флоату номинала*/
	amount, err := strconv.ParseFloat(strings.Replace(params.Get("amount"), ",", ".", 1), 64)
	if err != nil {
		resp.Code = status
		resp.Message = "Wrong amount passed. not float"
		resp.Response(w)
		return
	}
	firstRatio, _ := strconv.ParseFloat(strings.Replace(first.Ratio, ",", ".", 1), 64)
	secondRatio, _ := strconv.ParseFloat(strings.Replace(second.Ratio, ",", ".", 1), 64)
	/*Сама конвертация валют*/
	converted_amount := amount * firstRatio / secondRatio
	var res Data
	/*Приведение к виду ответа с данными*/
	res.Date = first.Date
	res.Source = first.Source
	res.First = first.Code
	res.Second = second.Code
	res.Amount = params.Get("amount")
	res.Converted_amount = strconv.FormatFloat(converted_amount, 'f', 3, 64)
	resp.Data = append(resp.Data, res)
	/*Вывод на страницу*/
	resp.Response(w)
}
func getAll(w http.ResponseWriter, r *http.Request) {
	/*Вывод в JSON */
	w.Header().Set("Content-Type", "application/json")
	var resp Response
	client, status, err := DbConn(w, r)
	/*Проверка на доступность бд */
	if err != nil {
		log.Printf("getAll, %s, %s \n", err.Error(), time.Now().In(loc))
		resp.Code = status
		resp.Message = err.Error()
		resp.Response(w)
		return
	}
	params, err := url.ParseQuery(r.URL.RawQuery)
	/*Проверка на правильность ввода параметров */
	if err != nil {
		log.Printf("getAll, %s, %s \n", err.Error(), time.Now().In(loc))
		resp.Code = http.StatusBadGateway
		resp.Message = "Wrong query"
		resp.Response(w)
		return
	}
	source := params.Get("source")
	/*Дефолтный источник*/
	if len(source) == 0 {
		source = def
	}
	var vals Vals
	var data_l []Data
	keys, _ := client.Keys(ctx, source+":*").Result()
	for _, v := range keys {
		_ = client.HGetAll(ctx, v).Scan(&vals)
		data_mar, _ := json.Marshal(vals)
		/*Приведение из структуры vals в Data*/
		data := new(Data)
		_ = json.Unmarshal(data_mar, data)
		data_l = append(data_l, *data)
	}
	if len(data_l) == 0 {
		resp.Code = http.StatusNotFound
		resp.Message = "Not found. Source" + source + "is wrong"
	} else {
		resp.Data = data_l
	}
	resp.Response(w)
}
func init() {
	if err := godotenv.Load("config.env"); err != nil {
		log.Print("No .env file found")
	}
}
func main() {
	key, f := os.LookupEnv("CB_TH_API_KEY")
	if !f {
		log.Fatal("No API key provided")
	}
	source_keys["TH"] = key
	red, f := os.LookupEnv("REDIS_DB")
	if !f {
		log.Fatal("No database provided")
	}
	redis_db = red

	http.HandleFunc("/", greet)
	http.HandleFunc("/getall", getAll)
	http.HandleFunc("/convert", convertCurrs)
	http.ListenAndServe(":8080", nil)
}
