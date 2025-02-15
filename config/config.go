// В config происходит инциализация переменных окружения из файла config.env. Источники отдельно изменять в данном файле.
// Обязательно нужна локация, иначе время будет неправильным
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Здесь находится вся нужная информация для корректной работы приложения
type AppConfig struct {
	//Ключи доступа к источникам
	SourceKeys map[string]string
	//Ссылки на источники
	SourceLinks map[string]string
	//Ссылка на подключение к бд
	DbUrl string
	//Максимальное количество переподключений к бд
	DbAttempts int
	//Коды источников
	Sources []string
	//Времена обновления источников
	SourceUpdates map[string]string
	//Локация времени
	Loc *time.Location
	//Время задержки обновления
	TimeoutUP int
	//Время задержки запросов в источники
	TimeoutREQ int
}

// Создание нового конфига. Достает данные из конфигурационного файла и использует методы getEnv* того же пакета
func NewAppConfig() *AppConfig {
	sourceKeys := getEnvWithPattern("SOURCE_KEY", map[string]string{"RU": "", "TH": ""})
	sourceLinks := getEnvWithPattern("SOURCE_LINK", map[string]string{"RU": "", "TH": ""})
	sourceUpdates := getEnvWithPattern("SOURCE_TIMES", map[string]string{"RU": "00:00:00", "TH": "18:00:00"})

	// Генерация списка источников
	sources := make([]string, 0, len(sourceUpdates))
	for k := range sourceUpdates {
		sources = append(sources, k)
	}

	return &AppConfig{
		SourceKeys:    sourceKeys,
		SourceLinks:   sourceLinks,
		DbUrl:         getEnv("DB_URL", ""),
		DbAttempts:    getEnvAsInt("DB_ATT", 5),
		Sources:       sources,
		Loc:           getEnvAsLoc("LOC", &time.Location{}),
		SourceUpdates: sourceUpdates,
		TimeoutUP:     getEnvAsInt("TIMEOUT_UP", 600),
		TimeoutREQ:    getEnvAsInt("TIMEOUT_REQ", 20),
	}
}

// Поиск переменной в файле config.env
func getEnv(key string, defaultVal string) string {
	if val, found := os.LookupEnv(key); found {
		return val
	}
	return defaultVal
}

// Поиск по ключу с добавлением кода страны (к примеру, SOURCE_KEY + RU = SOURCE_KEY_RU). Нужна для получения ссылки источника и ключа доступа (Если требуется)
func getEnvWithPattern(key string, defaultVal map[string]string) map[string]string {
	val := make(map[string]string, len(defaultVal))
	for k, v := range defaultVal {
		if envVal, found := os.LookupEnv(key + "_" + k); found {
			val[k] = envVal
		} else {
			val[k] = v
		}
	}
	return val
}

// Получение переменной в типе int по методу getEnv
func getEnvAsInt(key string, defaultVal int) int {
	valstr := getEnv(key, "")
	if val, err := strconv.Atoi(valstr); err == nil {
		return val
	}
	return defaultVal
}

// Получение переменной в типе time.Location по методу getEnv
func getEnvAsLoc(key string, defaultVal *time.Location) *time.Location {
	valstr := getEnv(key, "")
	if val, err := time.LoadLocation(valstr); err == nil {
		return val
	}
	return defaultVal
}

// Обработка строки времени. Если неправильно введено время (не в формате hh:mm:ss), то возвращается ошибка
func (app *AppConfig) ParseTime(source string) (time.Time, error) {
	time_in, err := time.ParseInLocation(time.TimeOnly, app.SourceUpdates[source], app.Loc)
	var err_resp error
	err_resp = nil
	if err != nil {
		err_resp = errors.New("wrong date in source " + source + ". write it in format hh:mm:ss")
	}
	return time_in, err_resp
}
