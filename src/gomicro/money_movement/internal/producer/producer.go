// Package producer для создания продюсера на отправки сообщений на Кафку
package producer

import (
	"encoding/json"
	"github.com/IBM/sarama"
	"log"
	"os"
	"sync"
	"time"
)

const (
	emailTopic = "email"
	ledgeTopic = "ledger"
)

type EmailMsg struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
}

type LedgerMsg struct {
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`
	Operation string `json:"operation"`
	Date      string `json:"date"`
}

func SendCaptureMessage(pid string, userID string, amount int64) {
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	// Создание синхронного продюсера (отправителя) Кафка (через библу Sarama)
	producer, err := sarama.NewSyncProducer([]string{"my-cluster-kafka-bootstrap:9092"}, sarama.NewConfig())
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Println(err)
		}
	}()

	// Создание сообщений для разных консюмеров:
	// Сообщение для е-мейл консюмера,
	emailMsg := EmailMsg{
		OrderID: pid,
		UserID:  userID,
	}

	// Сообщение для бухгалтерского консюмера
	ledgerMsg := LedgerMsg{
		OrderID:   pid,
		UserID:    userID,
		Amount:    amount,
		Operation: "DEBIT",
		Date:      time.Now().Format("2006-01-02"),
	}

	// Асинхронная отправка сообщений в очередь Кафки с помощью горутин
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go sendMsg(producer, emailMsg, emailTopic, wg)
	go sendMsg(producer, ledgerMsg, ledgeTopic, wg)
	wg.Wait()
}

func sendMsg[T EmailMsg | LedgerMsg](
	producer sarama.SyncProducer,
	msg T,
	topic string,
	wg *sync.WaitGroup) {
	defer wg.Done()

	// Перевод переданного сообщения (msg) в JSON (stringMsg)
	stringMsg, err := json.Marshal(msg)
	if err != nil {
		log.Println(err)
		return
	}

	// Создание сообщения для Кафки
	// message будет иметь тип структуры ProducerMessage из пакета sarama (тип для Кафки)
	message := &sarama.ProducerMessage{
		Topic: topic,                           // Топик определяет канал отправки
		Value: sarama.StringEncoder(stringMsg), // Перевод сообщения из JSON в формат для Кафки
	}

	// Отправляем сообщение на Кафку
	// partition вернет раздел топика в которое улетело сообщение
	// offset вернет позицию сообщения в партиции
	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Message sent to partition %d at offset %d\n", partition, offset)
}
