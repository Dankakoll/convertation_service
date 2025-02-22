package main

// Инциализация и подгрузка переменных окружения
// Нужна для отлова ошибок
// Чтобы запустить дебаггер вне докера надо заменить ссылку на дб на localhost:6380
// и подключить github.com/joho/godotenv
/*
func init() {
	err := godotenv.Load(".." + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "config" + string(os.PathSeparator) + "config.env")
	if err != nil {
		log.Fatal("No conf file provided. Check link")
	}

}
*/
