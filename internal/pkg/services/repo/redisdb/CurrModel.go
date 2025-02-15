package redisdb

import (
	"context"
	"errors"
	"log"
	"main/internal/pkg/domain"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Репозиторий хранения данных в бд Redis
type CurrModelRepository struct {
	conn       *redis.Client
	maxRetries int
}

// Создание нового репозитория. Нужен клиент redis
func NewCurrModelRepository(conn *redis.Client, maxRetries int) *CurrModelRepository {
	return &CurrModelRepository{conn, maxRetries}
}

// Проверка подключения. Если клиент отключен от бд, будет проведено переподключение к бд с таймаутом 1 секунда
func (r *CurrModelRepository) checkConn(ctx context.Context) error {
	if err := r.conn.Ping(ctx).Err(); err != nil {
		err = r.Reconnect(r.conn.Options().DialTimeout, ctx, r.maxRetries)
		if err != nil {
			return err
		}
	}
	return nil
}

// Получение данных по источнику и коду валюты. Ключи хранятся в виде "SOURCE:CODE"
func (r *CurrModelRepository) GetBySourceAndKey(ctx context.Context, source string, key string) (res domain.CurrModel, err error) {
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
func (r *CurrModelRepository) GetAllBySource(ctx context.Context, source string) (res []domain.CurrModel, err error) {
	if err := r.checkConn(ctx); err != nil {
		return res, err
	}
	keys, err := r.conn.Keys(ctx, source+":*").Result()
	if err != nil {
		return res, err
	}
	var vals domain.CurrModel
	if err := r.checkConn(ctx); err != nil {
		return res, err
	}
	for _, v := range keys {
		err := r.conn.HGetAll(ctx, v).Scan(&vals)
		if err != nil {
			return res, err
		}
		res = append(res, vals)
	}
	return res, nil
}

// Cохранение данных по источнику и коду валюты. В бд будет храниться по ключу "SOURCE:CODE"
func (r *CurrModelRepository) Store(ctx context.Context, curr domain.CurrModel) (err error) {
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
func (r *CurrModelRepository) Close(ctx context.Context) (err error) {
	return r.conn.Close()
}

// Reconnect переподключает к бд с интервалом 1 секунда. Прерывается и возвращает ненулевую ошибку при закрытии контекста.
func (r *CurrModelRepository) Reconnect(timeWait time.Duration, ctx context.Context, maxRetries int) (err error) {
	logger := log.New(os.Stdout, "Reconnect ", log.LstdFlags)
	logger.Printf("Checking connect")
	err = r.conn.Ping(ctx).Err()
	if err != nil {
		logger.Print("Connect with db was lost. Error: " + err.Error())
		attempt := 0
		ticker := time.NewTicker(timeWait)
		for range ticker.C {
			select {
			case <-ctx.Done():
				logger.Print("Gracefully stopping")
				return errors.New("exiting")
			default:
				if attempt > maxRetries {
					logger.Printf("%s", "Cannot connect to db after"+strconv.FormatInt(int64(attempt), 10)+" attempt. Will try again later")
					return errors.New("no connect with db now")
				}
				attempt++
				logger.Printf("Started reconnecting with DB")
				err = r.conn.Ping(ctx).Err()
				if err == nil {
					logger.Printf("Successfuly reconnected")
					return nil
				}
				logger.Printf("Reconnect failed. Waiting for %d sec.", timeWait)
			}

		}

	}
	logger.Printf("Connect is ok. Continuing to do business logic")
	return nil
}
