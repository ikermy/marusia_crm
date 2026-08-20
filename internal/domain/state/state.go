package state

var (
	UsersDB = make(chan struct{}) // Канал уведомления о завершении операций пользователями ДБ
	Exit    = make(chan struct{}) // Канал завершения работы приложения

	// TODO задел на потом
	// RedisAddr Redis — параметры подключения (заполняются в main.go из env).
	//RedisAddr     string // REDIS_ADDR (default: "" — Redis отключён)
	//RedisPassword string // REDIS_PASSWORD
	//RedisDB       int    // REDIS_DB (default: 0)
)
