// Package email содержит реализацию функций для email консюмера
package email

import (
	"fmt"
	"net/smtp"
	"os"
)

// Send отправляет сообщение о транзакции (orderID) клиенту target
func Send(target string, orderID string) error {
	// Данные отправителя
	senderEmail := os.Getenv("SENDER_EMAIL")
	password := os.Getenv("SENDER_PASSWORD")

	// Данные получателя
	recipientEmail := target

	// Данные SMTP (Simple Mail Transfer Protocol)
	smtpServer := "smtp.gmail.com"
	smtpPort := "587"
	smtpAddress := fmt.Sprintf("%s:%s", smtpServer, smtpPort)

	// Данные авторизации отправителя типа Auth
	creds := smtp.PlainAuth("", senderEmail, password, smtpServer)

	// Сообщение для отправки
	message := []byte(fmt.Sprintf("Subject: Payment Processed!\n Process ID: %s\n", orderID))

	// Отправка сообщения через протокол SMTP
	err := smtp.SendMail(smtpAddress, creds, senderEmail, []string{recipientEmail}, message)
	if err != nil {
		return err
	}

	return nil
}
