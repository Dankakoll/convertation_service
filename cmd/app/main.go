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
	"errors"
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

const ver = "4"

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
	log.Println("Starting API, Handler and DB service. Version:", ver)

	//Инициализация  апи хендлера
	APIHandler, err := handler.InitHandler(AppConfig, mainCtx)
	//Если произошла ошибка на инициализации
	if err != nil {
		log.Println("Error in intializing. Stopping immediatly. Check logs")
		os.Exit(1)
	}
	//Graceful shutdown
	stop_db := make(chan bool, 1)
	g, _ := errgroup.WithContext(mainCtx)
	//Горутина для выключения
	go APIHandler.StartUpdate(AppConfig, mainCtx)
	// Запуск сервера
	go func() {
		log.Println("Starting server")
		err := httpServer.ListenAndServe()
		//Если произошла ошибка при включении
		if !errors.Is(err, http.ErrServerClosed) && err != nil {
			log.Print("Error in working process. Starting shutdown")
			stop()
		}
	}()
	//Выключение сервера
	g.Go(func() error {
		//Ожидаем сигнал Sigterm
		<-mainCtx.Done()
		//Производим отключение с таймаутом
		log.Printf("Gracefully stopping server")
		shutdown_ctx, cancel := context.WithCancel(mainCtx)
		defer cancel()
		httpServer.Shutdown(shutdown_ctx)
		log.Printf("Server stopped")
		// Посылаем сигнал к бд
		stop_db <- true
		return http.ErrServerClosed
	})
	// Сначала отключаем сервер потом подключения к бд
	go func() error {
		for {
			for range stop_db {
				log.Printf("Closing connect with DB")
				return APIHandler.ExitConnectWithDb(mainCtx)
			}
		}
	}()
	if err := g.Wait(); err != nil {
		log.Printf("Service stopped")
	}

}
