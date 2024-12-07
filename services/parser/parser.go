// Parser реализует парсинг тела ответа в зависимости от типа данных.
package parser

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"main/domain"

	"golang.org/x/net/html/charset"
)

// Структура Parser
type Parser struct {
	datatype string
}

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
			err = errors.New("no source")
			return res, err
		}
	case "RU":
		toParse = new(domain.RUsourceDTO)
	case "TH":
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
					err = errors.New("no source")
					return res, err
				}
			case "TH":
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
