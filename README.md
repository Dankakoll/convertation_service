
# convertation_service
Сервис конвертации валют
Использовать методы /convert и /getall
Переменные окружения записывать в config/config.env
Архитектура описана в папке arch
## Зависимости
- [Docker](https://www.docker.com/)
- директория vendor

## Запуск приложения

### Сборка и запуск
```sh
docker compose up --build -d 
```
### Доступ

Доступ производится по порту 8080, подключение к бд инициировать в ссылке через db:6379 (см docker-compose.yaml)  

### Дальнейший запуск
```sh
docker compose up -d 
```
## Переменные окружения
| Название |  Описание |
| ----     | ---------- |
|LOC |  локация времени обновления данных  (оставить по умолчанию Asia/Bangkok)|
|SOURCES| коды источников (вписаны в config.go)|
|SOURCE_LINK_(RU,TH)| ссылки источников (обязательны)
|SOURCE_KEY_(RU,TH)| ключи доступа к источникам (обязательны)|
|DB_URL | ссылка подключения к бд в контейнере (обязателена)|
|TIMEOUT_UP | время задержки обновления данных (по умолчанию 600 с)|
|TIMEOUT_REQ | время задержки обновления данных (по умолчанию 20 с)|
|DB_ATT| количество попыток подключений к БД

# Документация
Генерируется кодом

```sh
swag init -g main.go --pd  -d ./cmd/app,./internal/pkg/services/handler

```
Результаты обработки swag лежат в директории docs
Возможно подключение к сервису как отдельной страницы

# Swagger Convertation_service API
Convertation_service for sources RU,TH

## Version: 1.0


**License:** [Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0.htm)

### /convert

#### GET
##### Summary:

Конвертация валют

##### Description:

Конвертация валют в зависимости от источника, требуется предоставление двух кодов валют, суммы конвертации, и курса обмена.

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| source | path | source | Yes | string |
| first | path | first | Yes | string |
| second | path | second | Yes | string |
| amount | path | amount | Yes | string |
| exchange | path | exchange | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OK | [handler.Response](#handler.Response) & object |
| 400 | Bad Request | [handler.Response](#handler.Response) |
| 404 | Not Found | [handler.Response](#handler.Response) |
| 500 | Internal Server Error | [handler.Response](#handler.Response) |
##### Examples
##### Request
```
http://127.0.0.1:8080/convert?source=TH&first=RUB&second=USD&amount=1000&exchange=buy
```
##### Successfull Response
```
{
  "code": 200,
  "message": "Conversion successful",
  "data": [
    {
      "date": "2025-02-20",
      "source": "RU",
      "first_curr": "RUB",
      "second_curr": "USD",
      "exchange": "buy",
      "amount": "1000",
      "converted_amount": "11.157763433929"
    }
  ]
}
```
##### Wrong Request
```
http://127.0.0.1:8080/convert?source=ABCD&first=RUB&second=USD&amount=1000&exchange=buy
```
##### Error Response
```
{
  "code": 400,
  "message": "wrong source ABCD provided"
}
```

### /getall

#### GET
##### Summary:

Получить все валюты

##### Description:

Получить все валюты из источника. Если источник не указан, берутся данные из источника по умолчанию (ЦБ РФ)

##### Parameters

| Name | Located in | Description | Required | Schema |
| ---- | ---------- | ----------- | -------- | ---- |
| source | path | source | Yes | string |

##### Responses

| Code | Description | Schema |
| ---- | ----------- | ------ |
| 200 | OK | [handler.Response](#handler.Response) & object |
| 400 | Bad Request | [handler.Response](#handler.Response) |
| 404 | Not Found | [handler.Response](#handler.Response) |
| 500 | Internal Server Error | [handler.Response](#handler.Response) |
##### Examples
##### Request
```
http://127.0.0.1:8080/getall?source=RU
```
##### Succsesfull  response
```
{
  "code": 200,
  "message": "Getting all queries from source RU successful",
  "data": [
    [
      {
        "date": "2025-02-22",
        "code": "BYN",
        "name": "Белорусский рубль",
        "ratio_buy": "27,4914",
        "ratio_sell": "27,4914"
      },
      {
      ...
      }
      ...
	  }
  ]
 ]
}
```
##### Wrong Request
```
http://127.0.0.1:8080/getall?source=ABCD
```

##### Error response
```
{
  "code": 500,
  "message": "wrong source provided"
}
```
### Models


#### api.ConvertResponse

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| amount | string |  | No |
| converted_amount | string |  | No |
| date | string |  | No |
| exchange | string |  | No |
| first_curr | string |  | No |
| second_curr | string |  | No |
| source | string |  | No |

#### domain.CurrModel

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| Code | string | Код валюты | No |
| Name | string | Полное название на языке из источника | No |
| RatioBuy | string | Курс покупки | No |
| RatioSell | string | Курс продажи | No |
| date | string | Дата получения валюты | No |

#### handler.Response

| Name | Type | Description | Required |
| ---- | ---- | ----------- | -------- |
| code | integer | Код ответа | No |
| data | [  ] | Данные | No |
| message | string | Сообщение для пользователя | No |
