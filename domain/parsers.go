// В пакете domain описаны основные структуры, с которыми работает бизнес-логика
package domain

import "encoding/xml"

//XML-структура для ЦБ РФ
type RUsourceDTO struct {
	XMLName xml.Name `xml:"ValCurs"`
	Date    string   `xml:"Date,attr"`
	Valute  []struct {
		CharCode  string `xml:"CharCode"`
		Name      string `xml:"Name" `
		Value     string `xml:"Value"`
		VunitRate string `xml:"VunitRate"`
	} `xml:"Valute"`
}

//JSON-стурктура для ЦБ Тайланда
type THsourceDTO struct {
	Result struct {
		Data struct {
			DataDetail []interface{} `json:"data_detail"`
		} `json:"data"`
	} `json:"result"`
}

//Раскрытие интерфейса DataDetail
type THsourceDTODataDetail struct {
	Period            string `json:"period"`
	CurrencyID        string `json:"currency_id"`
	Currency_Name_Eng string `json:"currency_name_eng"`
	Buying_Transfer   string `json:"buying_transfer"`
	Selling           string `json:"selling"`
}
