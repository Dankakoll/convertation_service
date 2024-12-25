// Пакет main запускает сервер и горутину обновления данных пакета handler и также подключается к пакету redisdb
// При получении сигнала SIGTERM останавливает работу сервера и отключается от базы данных
package main

//Аннотация Swagger

// @title Swagger Convertation_service API
// @version 1.0
// @description Convertation_service for sources RU,TH
// @host localhost:8080
// @BasePath /
// @produce json
// @query.collection.format multi
// @schemes http
// @externalDocs.description OpenAPI
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.htm
import (
	"context"
	"fmt"
	"log"
	"main/config"
	_ "main/docs"
	"main/internal/pkg/services/handler"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

func main() {
	AppConfig := config.NewAppConfig()
	//Инициализация сервера c контекстом. В контекст будет послан сигнал о выключении
	var mainCtx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr: ":8080",
		BaseContext: func(_ net.Listener) context.Context {
			return mainCtx
		},
	}

	go httpServer.ListenAndServe()

	log.Printf("Starting")

	//Инициализация  апи хендлера
	APIHandler := handler.InitHandler(AppConfig, mainCtx)

	//Graceful shutdown
	g, _ := errgroup.WithContext(mainCtx)
	//Горутина для выключения

	// Запуск горутины обновления данных
	g.Go(func() error {
		return APIHandler.StartUpdate(AppConfig, mainCtx)
	})

	//Запуск горутины отключения от бд
	g.Go(func() error {
		<-mainCtx.Done()
		return APIHandler.ExitConnectWithDb(mainCtx)
	})

	// Запуск сервера

	//Выключение сервера
	g.Go(func() error {
		//Если в канал контекста послан сигнал Sigterm
		<-mainCtx.Done()
		return httpServer.Shutdown(mainCtx)
	})
	//Если послан сигнал о выключении
	if err := g.Wait(); err != nil {
		fmt.Printf("exit reason: %s \n", err)
	}

}
