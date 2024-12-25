// В config происходит инциализация переменных окружения из файла config.env. Источники отдельно изменять в данном файле.
// Обязательно нужна локация, иначе время будет неправильным
package config

import (
	"log"
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
	return &AppConfig{
		SourceKeys:    getEnvWithPattern("SOURCE_KEY", map[string]string{"RU": "", "TH": ""}),
		SourceLinks:   getEnvWithPattern("SOURCE_LINK", map[string]string{"RU": "", "TH": ""}),
		DbUrl:         getEnv("DB_URL", ""),
		Sources:       []string{"RU", "TH"},
		Loc:           getEnvAsLoc("LOC", &time.Location{}),
		SourceUpdates: getEnvWithPattern("SOURCE_TIMES", map[string]string{"RU": "00:00:00", "TH": "18:00:00"}),
		TimeoutUP:     getEnvAsInt("TIMEOUT_UP", 600),
		TimeoutREQ:    getEnvAsInt("TIMEOUT_REQ", 20),
	}
}

func getEnv(key string, defaultVal string) string {
	if val, found := os.LookupEnv(key); found {
		return val
	}
	return defaultVal
}
func getEnvWithPattern(key string, defaultVal map[string]string) map[string]string {
	val := make(map[string]string, len(defaultVal))
	for k := range defaultVal {
		val[k] = getEnv(key+"_"+k, "")
		if len(val[k]) == 0 {
			val[k] = defaultVal[k]
		}
	}
	return val
}

func getEnvAsInt(key string, defaultVal int) int {
	valstr := getEnv(key, "")
	if val, err := strconv.Atoi(valstr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsLoc(key string, defaultVal *time.Location) *time.Location {
	valstr := getEnv(key, "")
	if val, err := time.LoadLocation(valstr); err == nil {
		return val
	}
	return defaultVal
}

func (app *AppConfig) ParseTime(source string) time.Time {
	time_in, err := time.ParseInLocation(time.TimeOnly, app.SourceUpdates[source], app.Loc)
	if err != nil {
		log.Fatalf("%s", "Wrong date in source "+source+". Write it in format hh:mm:ss")
	}
	return time_in
}
