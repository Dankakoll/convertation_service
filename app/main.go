// Пакет main запускает сервер и горутину обновления данных пакета handler и также подключается к пакету redisdb
// При получении сигнала SIGTERM останавливает работу сервера и отключается от базы данных
package main

import (
	"context"
	"fmt"
	"log"
	"main/config"
	"main/services/api"
	"main/services/handler"
	"main/services/repo/redisdb"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

/*
	func init() {
		_ = godotenv.Load("config.env")

}
*/
func main() {
	//Объявление переменных окружения
	AppConfig := config.NewAppConfig()
	//Время обновления
	var timeUpd = make(map[string]time.Time, len(AppConfig.Sources))
	for _, v := range AppConfig.Sources {
		timeUpd[v] = AppConfig.ParseTime(v)
	}
	//Инициализация бд
	opt, err := redis.ParseURL(AppConfig.Dburl)
	if err != nil {
		log.Fatal("Wrong link provided")
	}
	//Клиент базы данных
	var client = redis.NewClient(opt)
	//База данных
	var db = redisdb.NewValsDTORepository(client)

	//Инициализация сервера c контекстом. В контекст будет послан сигнал о выключении
	var mainCtx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr: ":8080",
		BaseContext: func(_ net.Listener) context.Context {
			return mainCtx
		},
	}

	log.Printf("Starting")
	//Инициализация  апи хендлера
	APIHandler := handler.NewAPIHandler(api.NewAPI(client, AppConfig.Source_keys, AppConfig.Source_links, AppConfig.TimeoutREQ, AppConfig.Loc, mainCtx))
	// Обработка запросов
	http.HandleFunc("/", APIHandler.Greet)
	http.HandleFunc("/getall", APIHandler.GetAll)
	http.HandleFunc("/convert", APIHandler.Convert)

	//Graceful shutdown

	g, gCtx := errgroup.WithContext(mainCtx)
	//Горутина для запуска сервера
	g.Go(func() error {
		return httpServer.ListenAndServe()
	})
	//Горутина для выключения
	g.Go(func() error {
		//Если в канал контекста послан сигнал Sigterm
		<-gCtx.Done()
		return httpServer.Shutdown(mainCtx)
	})
	//Горутина для отключения от бд
	g.Go(func() error {
		//Аналогично,отключение бд
		<-gCtx.Done()
		return db.Close(mainCtx)
	})
	g.Go(func() error {
		//Горутина для обновления данных
		return APIHandler.Update(AppConfig.TimeoutUP, AppConfig.Loc, timeUpd, AppConfig.Sources, gCtx)
	})

	//Если послан сигнал о выключении
	if err := g.Wait(); err != nil {
		fmt.Printf("exit reason: %s \n", err)
	}

	log.Printf("Exiting")

}
