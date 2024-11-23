package main

import (
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/*XML-структура для ЦБ РФ*/
type CBRVals struct {
	XMLName xml.Name `xml:"ValCurs"`
	Date    string   `xml:"Date,attr"`
	Valute  []struct {
		CharCode  string `xml:"CharCode"`
		Name      string `xml:"Name" `
		Value     string `xml:"Value"`
		VunitRate string `xml:"VunitRate"`
	} `xml:"Valute"`
}

/*JSON-стурктура для ЦБ Тайланда*/
type THVals struct {
	Result struct {
		Data struct {
			DataDetail []interface{} `json:"data_detail"`
		} `json:"data"`
	} `json:"result"`
}

/*Раскрытие интерфейса DataDetail*/
type THValsDataDetail struct {
	Period            string `json:"period"`
	Currency_Name_Eng string `json:"currency_name_eng"`
	CurrencyID        string `json:"currency_id"`
	MidRate           string `json:"mid_rate"`
}

/*Конструктор объекта Vals (хранится в бд)*/
func ToVals(Date string, source string, code string, name string, Ratio string) Vals {
	vals := new(Vals)
	vals.Date = Date
	vals.Source = source
	vals.Code = code
	vals.Name = name
	vals.Ratio = Ratio
	return *vals
}

func THsource(w http.ResponseWriter, r *http.Request) (int, error) {
	/*Добавление в бд данных с ЦБ Тайланда*/
	res, status, err := JSONparser("https://apigw1.bot.or.th/bot/public/Stat-ExchangeRate/v2/DAILY_AVG_EXG_RATE/", w, r)
	if err != nil {
		log.Printf("THsource, %s, %s \n", err.Error(), time.Now().In(loc))
		return status, err
	}
	source := res.(*THValsDataDetail)
	rg := regexp.MustCompile("[0-9]+")
	rg_s := rg.FindAllString(source.Currency_Name_Eng, -1)
	init_rate, _ := strconv.ParseFloat(strings.Replace(source.MidRate, ",", ".", 1), 64)
	/*Данные в этом источнике могут иметь отношение на определенный номинал. Далее идет нормализация
	(соотношение 1 бата к единице искомой валюты)*/
	if len(rg_s) != 0 {
		amount, _ := strconv.ParseInt(rg_s[0], 10, 16)
		source.MidRate = strconv.FormatFloat(init_rate/float64(amount), 'f', 5, 64)
	}
	vals := ToVals(source.Period, "TH", source.CurrencyID, source.Currency_Name_Eng, source.MidRate)
	client, status, err := DbConn(w, r)
	if err != nil {
		log.Printf("THsource, %s, %s \n", err.Error(), time.Now().In(loc))
		return status, err
	}
	err = client.HSet(ctx, "TH:"+vals.Code, vals).Err()
	if err != nil {
		log.Printf("THsource, %s, %s \n", "Wrong body is passed in TH source. Cannot write to db.", time.Now().In(loc))
		return http.StatusMethodNotAllowed, http.ErrNotSupported
	}
	return http.StatusOK, nil
}
func RUsource(w http.ResponseWriter, r *http.Request) (int, error) {
	/*Добавление в бд данных с ЦБ РФ*/
	res, status, err := XMLparser("http://www.cbr.ru/scripts/XML_daily.asp", w, r)
	if err != nil {
		log.Printf("RUsource, %s, %s \n", err.Error(), time.Now().In(loc))
		return status, err
	}
	source := res.(*CBRVals)
	splt := strings.Split(source.Date, ".")
	/*Приведение даты к формату yyyy-mm-dd*/
	new_date := splt[2] + "-" + splt[1] + "-" + splt[0]
	vals := make([]Vals, 0)
	for _, val := range source.Valute {
		vals = append(vals, ToVals(
			new_date, "RU",
			val.CharCode,
			val.Name,
			val.VunitRate))
	}
	client, status, err := DbConn(w, r)
	if err != nil {
		log.Printf("RUsource, %s, %s \n", err.Error(), time.Now().In(loc))
		return status, err
	}
	for _, v := range vals {
		/*Добавление в бд в соответствии с тэгами редис*/
		err = client.HSet(ctx, "RU:"+v.Code, v).Err()
		if err != nil {
			log.Printf("RUsource, %s, %s \n", "Wrong body is passed in RU source. Cannot write to db.", time.Now().In(loc))
			return http.StatusInternalServerError, http.ErrNotSupported
		}
	}
	return http.StatusOK, nil
}

func update(w http.ResponseWriter, r *http.Request) (int, error) {
	var status int
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		log.Printf("update, %s, %s \n", err.Error(), time.Now().In(loc))
		return http.StatusBadGateway, errors.New("Wrong query passed")
	}
	/*В зависимости от источника вызывается нужная функция*/
	if strings.Compare(params.Get("source"), "RU") == 0 {
		r.URL.RawQuery = ""
		status, err = RUsource(w, r)
	} else if strings.Compare(params.Get("source"), "TH") == 0 {
		r.URL.RawQuery = params.Encode()
		status, err = THsource(w, r)
	} else {
		/*Введен неправильный источник*/
		return http.StatusNotFound, errors.New("No source found")
	}
	if err != nil {
		log.Printf("update, %s, %s \n", err.Error(), time.Now().In(loc))
		return status, err
	}
	return http.StatusOK, nil
}
