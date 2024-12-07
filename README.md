
# convertation_service
Сервис конвертации валют
Использовать методы /convert и /getall
Переменные окружения записывать в config.env
Архитектура описана в папке arch
## Зависимости
- [Docker](https://www.docker.com/)

## Запуск приложения

### Сборка и запуск
```sh
docker compose up --build
```
### Доступ

Доступ производится по порту 8080, подлкючение к бд инициировать в ссылке через db:6379 (см compose.yaml)  

### Дальнейший запуск
```sh
docker compose up 
```
## Переменные окружения
LOC - локация времени обновления данных  (оставить по умолчанию Asia/Bangkok)
SOURCES - коды источников (вписаны в config.go)
SOURCE_LINK_(RU,TH) - ссылки источников (обязательны)
SOURCE_KEY_(RU,TH) - ключи доступа к источникам (обязательны)
DB_URL - ссылка подключения к бд в контейнере (обязателена)
TIMEOUT_UP - время задержки обновления данных (по умолчанию 600 с)
TIMEOUT_REQ - время задержки обновления данных (по умолчанию 20 с)


# APIDog
---
title: convertation_service
language_tabs:
  - shell: Shell
  - http: HTTP
  - go: Go
toc_footers: []
includes: []
search: true
code_clipboard: true
highlight_theme: darkula
headingLevel: 2
generator: "@tarslib/widdershins v4.0.23"

---

Base URLs:
```
http://127.0.0.1:8080
```
# Default

## GET getall

GET /getall

### Params

|Name|Location|Type|Required|Description|
|---|---|---|---|---|
|source|query|string| no |Type of source RU(cbr.ru) or TH(apiportal.bot.or.th)|

> Request Example
```sh
 curl -XGET 'http://127.0.0.1:8080/getall?source=REPLACE(RU,TH,"")
 ``` 
> Response Examples

> 200 Response

```json
{
  "data": [
    [
      {
        "date": "2024-12-06",
        "Code": "KRW",
        "Name": "SOUTH KOREA : WON (KRW) ",
        "RatioBuy": "0.0241000",
        "RatioSell": "0.0242000"
      },
      {
        "date": "2024-12-06",
        "Code": "USD",
        "Name": "USA : DOLLAR (USD) ",
        "RatioBuy": "34.0856000",
        "RatioSell": "34.2503000"
      },
	...
    ]
  ]
}

```

### Responses

|HTTP Status Code |Meaning|Description|Data schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|none|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|none|Inline|

### Responses Data Schema

HTTP Status Code **200**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|string|false|none||none|
|» message|string|false|none||none|
|» data|[object]|false|none||none|
|»» date|string|true|none||none|
|»» code|string|true|none||none|
|»» name|string|true|none||none|
|»» ratio|string|true|none||none|

HTTP Status Code **400**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|integer|true|none||none|
|» message|string|true|none||none|

HTTP Status Code **404**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|integer|true|none||none|
|» message|string|true|none||none|

# curr

## GET convert

GET /convert

convert

### Params

|Name|Location|Type|Required|Description|
|---|---|---|---|---|
|source|query|string| yes |Type of source RU(cbr.ru) or TH(apiportal.bot.or.th)|
|first|query|string| yes  |First currency|
|second|query|string| yes |Second currency|
|amount|query|string| yes  |Amount to convert, must contain numbers|
|course|query|string| yes  |Course of convert (buy,sell)|
> Request Example
```sh
 curl -XGET 'http://127.0.0.1:8080/convert?source=REPLACE("RU","TH")&first=REPLACE&second=REPLACE&amount=REPLACE&cource=REPLACE("buy","sell")' 
```
> Response Examples

```json
{
  "data": [
    {
      "date": "2024-12-07",
      "source": "RU",
      "first_curr": "USD",
      "second_curr": "USD",
      "course": "buy",
      "amount": "1000",
      "converted_amount": "1000.000"
    }
  ]
}	
```

> 400 Response

```json
{
  "code": 400,
  "message": "string"
}
```

### Responses

|HTTP Status Code |Meaning|Description|Data schema|
|---|---|---|---|
|200|[OK](https://tools.ietf.org/html/rfc7231#section-6.3.1)|none|Inline|
|400|[Bad Request](https://tools.ietf.org/html/rfc7231#section-6.5.1)|none|Inline|
|404|[Not Found](https://tools.ietf.org/html/rfc7231#section-6.5.4)|none|Inline|

### Responses Data Schema

HTTP Status Code **200**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|integer|true|none||none|
|» message|string|true|none||none|
|» data|[object]|true|none||none|
|»» date|string|false|none||none|
|»» source|string|true|none||none|
|»» first_curr|string|true|none||none|
|»» second_curr|string|true|none||none|
 »» course|string|true|none||none|
|»» amount|string|false|none||none|
|»» converted_amount|string|false|none||none|

HTTP Status Code **400**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|integer|true|none||none|
|» message|string|true|none||none|

HTTP Status Code **404**

|Name|Type|Required|Restrictions|Title|description|
|---|---|---|---|---|---|
|» code|integer|true|none||none|
|» message|string|true|none||none|


