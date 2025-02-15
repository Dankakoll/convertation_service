// Parser реализует парсинг тела ответа в зависимости от типа данных.
package parser

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log"
	"main/internal/pkg/domain"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
)

const SourceRU = "RU"
const SourceTH = "TH"

// Структура Parser
type Parser struct {
	datatype string
}

var logger = log.New(os.Stdout, "Parser ", log.LstdFlags|log.Lshortfile)

// Создание нового Parser. Parser реализует парсинг тела ответа в зависимости от типа данных. Нужен тип данных тела ответа
func NewParser(datatype string) *Parser {
	return &Parser{datatype}
}

// Метод Parser. В зависимости от тела ответа и источника идет приведение к структурам пакета domain.
// Возвращает ненулевую ошибку если тело пусто или такого источника нет
func (p *Parser) Parse(source string, body []byte) (res interface{}, err error) {
	var toParse interface{}

	//Поиск по источнику
	switch source {
	default:
		{
			err = errors.New("wrong source provided")
			return res, err
		}
	case SourceRU:
		toParse = new(domain.RUsourceDTO)
	case SourceTH:
		toParse = new(domain.THsourceDTO)
	}

	switch p.datatype {
	case "JSON":
		{
			err = json.Unmarshal(body, &toParse)
			if err != nil {
				return res, err
			}
			switch source {
			default:
				{
					err = errors.New("wrong source provided")
					return res, err
				}
			case SourceTH:
				{
					//Данный источник имеет вложенную структуру,
					//требуется дополнительное объявление еще одного объекта

					var toParseDataDetail domain.THsourceDTODataDetail
					DataDetail, _ := json.Marshal(toParse.(*domain.THsourceDTO).Result.Data.DataDetail[0])
					err = json.NewDecoder(bytes.NewReader(DataDetail)).Decode(&toParseDataDetail)
					if err != nil {
						return res, err
					}
					res = toParseDataDetail
				}

			}
		}
	case "XML":
		{
			d := xml.NewDecoder(bytes.NewReader(body))
			d.CharsetReader = charset.NewReaderLabel
			err = d.Decode(&toParse)
			if err != nil {
				return res, err
			}
			res = toParse
		}
	}
	return res, nil
}

func (p *Parser) ParseCurrtoDTO(newCurr interface{}, source string) (dom []domain.CurrModel, err error) {
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
				logger.Printf("Nil data in source RU. err: " + err.Error())
				return []domain.CurrModel{}, errors.New("parsed nil data from source RU. abort")
			}
			for _, curr := range RUDTO.Valute {
				dom = append(dom, domain.ToCurrModel(
					newDate, SourceRU,
					curr.CharCode,
					curr.Name,
					curr.VunitRate, curr.VunitRate))
			}
		}
	case SourceTH:
		{

			THDTO := newCurr.(domain.THsourceDTODataDetail)
			rg := regexp.MustCompile("[0-9]+")
			rgS := rg.FindAllString(THDTO.CurrencyNameEng, -1)
			initRateBuy, err := strconv.ParseFloat(strings.Replace(THDTO.BuyingTransfer, ",", ".", 1), 64)
			if err != nil {
				logger.Println("wrong RatioBuy in source TH for curr " + THDTO.CurrencyNameEng + ". Abort " + err.Error())
				return []domain.CurrModel{}, errors.New("wrong RatioBuy in source TH for curr " + THDTO.CurrencyNameEng + ". Abort")
			}
			initRateSell, err := strconv.ParseFloat(strings.Replace(THDTO.Selling, ",", ".", 1), 64)
			if err != nil {
				return []domain.CurrModel{}, errors.New("wrong RatioSell in source TH " + THDTO.CurrencyNameEng + ". Abort")
			}
			//Данные в этом источнике могут иметь отношение на определенный номинал. Далее идет нормализация
			//(соотношение 1 бата к единице искомой валюты)
			if len(rgS) != 0 {
				amount, err := strconv.ParseInt(rgS[0], 10, 16)
				if err != nil {
					logger.Println("wrong amount of currenct in source TH " + THDTO.CurrencyNameEng + ". Abort. err:" + err.Error())
					return []domain.CurrModel{}, errors.New("wrong amount of currenct in source TH " + THDTO.CurrencyNameEng + ". Abort")
				}
				THDTO.BuyingTransfer = strconv.FormatFloat(initRateBuy/float64(amount), 'f', 5, 64)
				THDTO.Selling = strconv.FormatFloat(initRateSell/float64(amount), 'f', 5, 64)
			}
			//Случай получения пустых данных
			if len(THDTO.Period) == 0 {
				logger.Println("parsed nil data from source TH. abort")
				return []domain.CurrModel{}, errors.New("parsed nil data from source TH. abort")
			}
			dom = append(dom, domain.ToCurrModel(THDTO.Period, SourceTH,
				THDTO.CurrencyID, THDTO.CurrencyNameEng,
				THDTO.BuyingTransfer, THDTO.Selling))
			if err != nil {
				logger.Println("Error occured while updating currency " + THDTO.CurrencyNameEng + "in source TH. Err:" + err.Error())
				return []domain.CurrModel{}, errors.New("error occured while updating currency " + THDTO.CurrencyNameEng + "in source TH. abort")
			}
		}
	default:
		return dom, errors.New("wrong source " + source + " provided")
	}
	return dom, nil
}
