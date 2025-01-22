package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sunr3d/gomicro/internal/ledger"
	"log"
	"os"
	"sync"
)

const (
	topic    = "ledger"
	dbDriver = "mysql"  // Драйвер базы данных
	dbName   = "ledger" // Имя базы данных
)

var (
	db *sql.DB
	wg sync.WaitGroup
)

type LedgerMsg struct {
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`
	Operation string `json:"operation"`
	Date      string `json:"date"`
}

func main() {
	var err error                             // переменная ошибки
	dbUser := os.Getenv("MYSQL_USER")         // Имя пользователя БД
	dbPassword := os.Getenv("MYSQL_PASSWORD") // Пароль (!ВАЖНО: никогда не хранить так в реальном проекте!)
	/// БЛОК DataBase(!)
	// Формирование строки подключения к БД (dsn = Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(mysql-ledger:3306)/%s", dbUser, dbPassword, dbName)

	// Открытие соединения с базой данных
	db, err = sql.Open(dbDriver, dsn)
	if err != nil {
		log.Fatalln(err) // Завершение программы при ошибке подключения
	}

	// Отложенное закрытие соединения с БД через анонимную функцию
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing DB: %s", err)
		}
	}()

	// Проверка работоспособности соединения
	err = db.Ping()
	if err != nil {
		log.Fatalln(err) // Завершение программы при отсутствии связи
	}
	/// БЛОК DataBase(!)

	sarama.Logger = log.New(os.Stdout, "[sarama]", log.LstdFlags)
	done := make(chan struct{})
	// Создание Кафка консюмера (kafka:9092 будет в докер окружении)
	consumer, err := sarama.NewConsumer([]string{"my-cluster-kafka-bootstrap:9092"}, sarama.NewConfig())
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		close(done)
		if err := consumer.Close(); err != nil {
			log.Println(err)
		}
	}()

	// Получаем лист партиций из конкретного раздела (в нашем случае дефайнд как email)
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		log.Fatalln(err)
	}

	// Т.к. партиций может быть несколько
	// Создаем цикл в котором из списка партиций забираем каждую по отдельности с помощью партишнКонсюмера
	// для обработки сообщений используется функция awaitMessages
	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			log.Fatalln(err)
		}

		defer func() {
			if err := pc.Close(); err != nil {
				log.Println(err)
			}
		}()

		wg.Add(1)
		go awaitMessages(pc, partition, done)
	}

	wg.Wait()

}

// Функция обработки и ожидания сообщений из Кафки
// Горутина висит бесконечно в ожидании пока не придет сообщение или сигнал Дон
func awaitMessages(pc sarama.PartitionConsumer, partition int32, done chan struct{}) {
	defer wg.Done()

	for {
		select {
		// Если в канал Дон приходит сигнал - завершаем горутину
		case <-done:
			fmt.Printf("Done signal've been received. Exiting...")
			return
		// Считываем сообщение из партишнКонсюмера в перемен msg
		case msg := <-pc.Messages():
			fmt.Printf("Partition %d - Received message: %s\n", partition, string(msg.Value))
			// Обработка считанного сообщения
			handleMessage(msg)
		}
	}
}

// Функция обработки сообщения из партиции Кафки
func handleMessage(msg *sarama.ConsumerMessage) {
	// Перевод сообщения из формата JSON в Го-структуру
	var ledgerMsg LedgerMsg
	if err := json.Unmarshal(msg.Value, &ledgerMsg); err != nil {
		fmt.Println(err)
		return
	}

	// Отправка сообщения в Леджер через функцию Insert из кастомного пакета ledger
	err := ledger.Insert(db, ledgerMsg.OrderID, ledgerMsg.UserID, ledgerMsg.Amount, ledgerMsg.Operation, ledgerMsg.Date)
	if err != nil {
		fmt.Println(err)
		return
	}
}
