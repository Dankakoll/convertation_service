# convertation_service
Сервис конвертации валют
Использовать методы /convert и /getall
В конфигурационный файл указать ссылку на базу данных redis (локальная база -- docker-compose) и ключ авторизации в источник
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

# Authentication

# Default

## GET getall

GET /getall

### Params

|Name|Location|Type|Required|Description|
|---|---|---|---|---|
|source|query|string| no |Type of source RU(cbr.ru) or TH(apiportal.bot.or.th)|

> Response Examples

> 200 Response

```json
{
  "code": "string",
  "message": "string",
  "data": [
    {
      "date": "string",
      "code": "string",
      "name": "string",
      "ratio": "string"
    }
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
|source|query|string| no |Type of source RU(cbr.ru) or TH(apiportal.bot.or.th)|
|first|query|string| no |First currency|
|second|query|string| no |Second currency|
|amount|query|string| no |Amount to convert, must contain numbers|

> Response Examples

```json
{
  "data": [
    {
      "date": "2024-11-22",
      "source": "RU",
      "first_curr": "USD",
      "second_curr": "USD",
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
|»» source|string|false|none||none|
|»» first_curr|string|false|none||none|
|»» second_curr|string|false|none||none|
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

# Data Schema

