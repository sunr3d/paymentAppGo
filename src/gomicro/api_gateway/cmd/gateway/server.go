package main

import (
	"context"
	"encoding/json"
	"fmt"
	authpb "github.com/sunr3d/gomicro/auth"
	mmpb "github.com/sunr3d/gomicro/money_movement"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"net/http"
	"strings"
)

var (
	mmClient   mmpb.MoneyMovementServiceClient
	authClient authpb.AuthServiceClient
)

func main() {
	// Создание гРПС конекшена для сервиса Аутентификации
	authConn, err := grpc.NewClient("auth:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := authConn.Close(); err != nil {
			log.Println(err)
		}
	}()

	// Создание клиента Транзакций из установленного конекшена
	authClient = authpb.NewAuthServiceClient(authConn)

	// Создание гРПС конекшена для сервиса Транзакций
	mmConn, err := grpc.NewClient("money_movement:7000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := mmConn.Close(); err != nil {
			log.Println(err)
		}
	}()

	// Создание клиента Аутентификации из установленного конекшена
	mmClient = mmpb.NewMoneyMovementServiceClient(mmConn)

	http.HandleFunc("/login", login)
	http.HandleFunc("/customer/payment/auth", customerPaymentAuth)
	http.HandleFunc("/customer/payment/capture", customerPaymentCapture)

	fmt.Println("listening on port 8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln(err)
	}
}

// Описание хендлера login
func login(w http.ResponseWriter, r *http.Request) {
	// Получение логина и пароля через стандартный http метод BasicAuth()
	// Если нет совпадений, то возвращаем ошибку 401
	userName, password, ok := r.BasicAuth()
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// gRPC методом GetToken получаем токен пользователя
	// При ошибке с сервера в ответ записываем ошибку
	ctx := context.Background()
	token, err := authClient.GetToken(ctx, &authpb.Credentials{
		UserName: userName,
		Password: password,
	})
	if err != nil {
		_, writeErr := w.Write([]byte(err.Error()))
		if writeErr != nil {
			log.Println(writeErr)
		}
		return
	}
	// При успешном получении токена возвращаем ответ 200 и токен
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(token.Jwt))
	if err != nil {
		log.Println(err)
	}
}

// Описание хендлера авторизации платежа
func customerPaymentAuth(w http.ResponseWriter, r *http.Request) {
	// 1. Блок проверки Authorization заголовка
	// Получаем заголовок Authorization стандартным http методом Header.Get()
	// Если заголовок пустой, то с сервера возвращаем ошибку 401
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Проверяем есть ли в заголовке префикс Bearer
	// При отсутствии возвращаем с сервера ошибку 401
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Извлекаем стринговый токен вырезая из него "Bearer " (он нам не понадобится)
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. Блок валидации токена
	ctx := context.Background()
	// Валидируем токен gRPC методом ValidateToken
	// При ошибке с сервера отправляем ответ 401
	_, err := authClient.ValidateToken(ctx, &authpb.Token{Jwt: token})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// 3. Блок десериализации payload
	// Объявление и создание го-структуры для десериализации JSON пейлоада
	type authorizePayload struct {
		CustomerWalletUserID string `json:"customer_wallet_user_id"`
		MerchantWalletUserID string `json:"merchant_wallet_user_id"`
		Cents                int64  `json:"cents"`
		Currency             string `json:"currency"`
	}
	var payload authorizePayload

	// Читаем тело запроса в поле body
	// При ошибке возвращаем 500
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Переводим JSON из тела хттп запроса в нашу го-структуру
	// При ошибке возвращаем 500
	err = json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// 4. Блок авторизации платежа
	ctx = context.Background()
	// Авторизуем транзакцию gRPC методом Authorize (money_movement)
	// При ошибке записываем в ответ текст ошибки
	ar, err := mmClient.Authorize(ctx, &mmpb.AuthorizePayload{
		CustomerWalletUserID: payload.CustomerWalletUserID,
		MerchantWalletUserID: payload.MerchantWalletUserID,
		Cents:                payload.Cents,
		Currency:             payload.Currency,
	})
	if err != nil {
		_, writeErr := w.Write([]byte(err.Error()))
		if writeErr != nil {
			log.Println(writeErr)
		}
		return
	}

	// 5. Блок формирования ответа
	// Создание Го-структуры ответа с айдишником транзакции
	type response struct {
		Pid string `json:"pid"`
	}
	// Записываем в го-структуру ответа айди транзакции из поля авторизации
	resp := response{
		Pid: ar.Pid,
	}

	// Переводим го-структуру в JSON формат
	// При ошибке сериализации возвращаем 500
	responseJSON, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// При успешной авторизации платежа отправляем с сервера
	// Код 200 и JSON с ответом (айди транзакции)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println(err)
		return
	}
}

// Описание хендлера "захвата" платежа
func customerPaymentCapture(w http.ResponseWriter, r *http.Request) {
	// 1. Блок проверки Authorization заголовка
	// Получаем заголовок Authorization стандартным http методом Header.Get()
	// Если заголовок пустой, то с сервера возвращаем ошибку 401
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Проверяем есть ли в заголовке префикс Bearer
	// При отсутствии возвращаем с сервера ошибку 401
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Извлекаем стринговый токен вырезая из него "Bearer " (он нам не понадобится)
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. Блок валидации токена
	ctx := context.Background()
	// Валидируем токен gRPC методом ValidateToken
	// При ошибке с сервера отправляем ответ 401
	_, err := authClient.ValidateToken(ctx, &authpb.Token{Jwt: token})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// 3. Блок десериализации payload
	// Объявление и создание го-структуры для десериализации JSON пейлоада
	type capturePayload struct {
		Pid string `json:"pid"`
	}
	var payload capturePayload

	// Читаем тело запроса в поле body
	// При ошибке возвращаем 500
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Переводим JSON из тела хттп запроса в нашу го-структуру
	// При ошибке возвращаем 500
	err = json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// 4. Блок авторизации платежа
	ctx = context.Background()
	// Захватываем транзакцию gRPC методом Capture (money_movement)
	// При ошибке записываем в ответ текст ошибки
	_, err = mmClient.Capture(ctx, &mmpb.CapturePayload{Pid: payload.Pid})
	if err != nil {
		_, writeErr := w.Write([]byte(err.Error()))
		if writeErr != nil {
			log.Println(writeErr)
		}
		return
	}

	// При успешном захвате возвращаем статус 200
	w.WriteHeader(http.StatusOK)
}
