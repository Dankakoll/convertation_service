package redisdb

import (
	"context"
	"errors"
	"log"
	"main/domain"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Репозиторий хранения данных в бд Redis
type ValsDTORepository struct {
	conn *redis.Client
}

// Создание нового репозитория. Нужен клиент redis
func NewValsDTORepository(conn *redis.Client) *ValsDTORepository {
	return &ValsDTORepository{conn}
}

// Проверка подключения. Если клиент отключен от бд, будет проведено переподключение к бд с таймаутом 1 секунда
func (r *ValsDTORepository) checkConn(ctx context.Context) error {
	if err := r.conn.Ping(ctx).Err(); err != nil {
		err = r.Reconnect(r.conn.Options().DialTimeout, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Получение данных по источнику и коду валюты. Ключи хранятся в виде "SOURCE:CODE"
func (r *ValsDTORepository) GetBySourceAndKey(ctx context.Context, source string, key string) (res domain.ValsDTO, err error) {
	if err := r.checkConn(ctx); err != nil {
		return res, err
	}
	err = r.conn.HGetAll(ctx, source+":"+key).Scan(&res)
	if err != nil {
		log.Fatal(err)
		return res, err
	}
	return res, nil
}

// Получение данных по источнику и коду валюты.
func (r *ValsDTORepository) GetAllBySource(ctx context.Context, source string) (res []domain.ValsDTO, err error) {
	if err := r.checkConn(ctx); err != nil {
		return res, err
	}
	keys, err := r.conn.Keys(ctx, source+":*").Result()
	if err != nil {
		return res, err
	}
	var vals domain.ValsDTO
	for _, v := range keys {
		if err := r.checkConn(ctx); err != nil {
			return res, err
		}
		err := r.conn.HGetAll(ctx, v).Scan(&vals)
		if err != nil {
			return res, err
		}
		res = append(res, vals)
	}
	return res, nil
}

// Cохранение данных по источнику и коду валюты. В бд будет храниться по ключу "SOURCE:CODE"
func (r *ValsDTORepository) Store(ctx context.Context, curr domain.ValsDTO) (err error) {
	if err := r.checkConn(ctx); err != nil {
		return err
	}
	key := curr.Source + ":" + curr.Code
	err = r.conn.HSet(ctx, key, curr).Err()
	if err != nil {
		return err
	}
	return nil
}

// Закрытие подключения к бд. Реализуется в main через горутину graceful shutdown
func (r *ValsDTORepository) Close(ctx context.Context) (err error) {
	return r.conn.Close()
}

// Reconnect переподключает к бд с интервалом 1 секунда. Прерывается и возвращает ненулевую ошибку при закрытии контекста.
func (r *ValsDTORepository) Reconnect(time_wait time.Duration, ctx context.Context) (err error) {
	logger := log.New(os.Stdout, "Reconnect ", log.LstdFlags)
	logger.Printf("Checking connect")
	err = r.conn.Ping(ctx).Err()
	timeout := 1
	if err != nil {
		logger.Print("Connect with db was lost. Error: " + err.Error())
		attempt := 0
		ticker := time.NewTicker(time.Duration(timeout) * time.Second)
		for range ticker.C {
			select {
			case <-ctx.Done():
				return errors.New("exiting")
			default:
				if attempt > r.conn.Options().MaxRetries {
					logger.Printf("Cannot connect to db after" + strconv.FormatInt(int64(attempt), 10) + " attempt. Will try again later")
					return errors.New("no connect with db now")
				}
				attempt++
				logger.Printf("Started reconnecting with DB")
				err = r.conn.Ping(ctx).Err()
				if err == nil {
					logger.Printf("Successfuly reconnected")
					return nil
				}
				logger.Printf("Reconnect failed. Waiting for %d sec.", timeout)
			}

		}

	}
	logger.Printf("Connect is ok. Doing business logic")
	return nil
}
