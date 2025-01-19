package main

import (
	"database/sql" // Стандартный пакет для работы с SQL базами данных
	"fmt"          // Пакет для форматированного ввода/вывода
	_ "github.com/go-sql-driver/mysql"
	mm "github.com/sunr3d/gomicro/internal/implementation"
	pb "github.com/sunr3d/gomicro/proto" // Протобаф сервис для gRPC
	"google.golang.org/grpc"             // Библиотека для gRPC
	"log"                                // Пакет логирования
	"net"
	"os"
)

// Константы подключения к БД (дефайны)
const (
	dbDriver = "mysql"          // Драйвер базы данных
	dbName   = "money_movement" // Имя базы данных
)

var db *sql.DB // Глобал переменная для базы данных

func main() {
	var err error                             // переменная ошибки
	dbUser := os.Getenv("MYSQL_USER")         // Имя пользователя БД
	dbPassword := os.Getenv("MYSQL_PASSWORD") // Пароль (!ВАЖНО: никогда не хранить так в реальном проекте!)
	/// БЛОК DataBase(!)
	// Формирование строки подключения к БД (dsn = Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(mysql-money-movement:3306)/%s", dbUser, dbPassword, dbName)

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

	/// БЛОК gRPC SERVER(!)
	// Создание нового ПУСТОГО gRPC сервера
	grpcServer := grpc.NewServer()
	pb.RegisterMoneyMovementServiceServer(grpcServer, mm.NewMoneyMovementImplementation(db))

	listener, err := net.Listen("tcp", ":7000")
	if err != nil {
		log.Fatalf("failed to listen on port 7000: %v\n", err)
	}

	// Логирование адреса сервера
	log.Printf("server listening at %v\n", listener.Addr())

	// Запуск gRPC сервера
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve grpc server: %v\n", err)
	}
	/// БЛОК gRPC SERVER(!)
}
