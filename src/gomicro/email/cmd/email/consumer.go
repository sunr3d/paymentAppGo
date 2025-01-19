// Кафка консюмер для email сервиса по отправке сообщения о платеже клиенту на почту

package main

import (
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/sunr3d/gomicro/internal/email"
	"log"
	"os"
	"sync"
)

const topic = "email"

var wg sync.WaitGroup

type EmailMsg struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
}

func main() {
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
		// Считываем сообщение из партишн Консюмера в перемен msg
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
	var emailMsg EmailMsg
	if err := json.Unmarshal(msg.Value, &emailMsg); err != nil {
		fmt.Println(err)
		return
	}

	// Отправка сообщения клиенту по е-мейл через функцию Send из кастомного пакета email
	err := email.Send(emailMsg.UserID, emailMsg.OrderID)
	if err != nil {
		fmt.Println(err)
		return
	}
}
