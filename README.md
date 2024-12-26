
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

Доступ производится по порту 8080, подлкючение к бд инициировать в ссылке через db:6379 (см compose.yaml)  

### Дальнейший запуск
```sh
docker compose up -d 
```
## Переменные окружения
LOC - локация времени обновления данных  (оставить по умолчанию Asia/Bangkok)
SOURCES - коды источников (вписаны в config.go)
SOURCE_LINK_(RU,TH) - ссылки источников (обязательны)
SOURCE_KEY_(RU,TH) - ключи доступа к источникам (обязательны)
DB_URL - ссылка подключения к бд в контейнере (обязателена)
TIMEOUT_UP - время задержки обновления данных (по умолчанию 600 с)
TIMEOUT_REQ - время задержки обновления данных (по умолчанию 20 с)

#Документация
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

### /getAll

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