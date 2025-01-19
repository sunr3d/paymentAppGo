// Package auth реализует сервис авторизации пользователя
package auth

import (
	"context"
	"database/sql"
	"errors"
	jwt "github.com/golang-jwt/jwt/v5"
	pb "github.com/sunr3d/gomicro/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"os"
	"time"
)

// Структура Implementation implements AuthServiceServer
type Implementation struct {
	db                                *sql.DB // Подключение к базе данных
	pb.UnimplementedAuthServiceServer         // Заглушка для совместимости
}

// Конструктор (функция) для создания новой реализации сервиса (принимает БД, возвращает объект Implementation)
func NewAuthImplementation(db *sql.DB) *Implementation {
	return &Implementation{db: db}
}

// Метод для получения токена с аутентификацией пользователя
// Сигнатура метода:
// - ctx: контекст выполнения запроса
// - credentials: структура с учетными данными пользователя
// Возвращает токен или ошибку
func (this *Implementation) GetToken(ctx context.Context, credentials *pb.Credentials) (*pb.Token, error) {
	// Локальная структура для хранения данных пользователя из БД
	type user struct {
		userID   string // Email пользователя
		password string // Пароль пользователя
	}
	// Создание экземпляра структуры для загрузки данных
	var u user

	// Подготовка SQL-запроса с безопасными плейсхолдерами.
	// Ищем пользователя по логину и паролю
	stmt, err := this.db.Prepare("SELECT user_id, password FROM users WHERE user_id = ? AND password = ?")

	// Обработка ошибки подготовки запроса
	if err != nil {
		// Логирование технической ошибки
		log.Println(err)
		// Возврат внутренней ошибки сервера
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	// Выполнение запроса:
	// - Подставляем логин и пароль из credentials
	// - Загружаем результат в структуру пользователя
	err = stmt.QueryRow(credentials.GetUserName(), credentials.GetPassword()).Scan(&u.userID, &u.password)

	// Обработка ошибок выполнения запроса
	if err != nil {
		// Если пользователь не найден
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		// Для других ошибок - внутренняя ошибка сервера
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Генерация JWT токена для пользователя
	jwtToken, err := createJWT(u.userID)
	if err != nil {
		return nil, err
	}

	// Возврат токена в протобуферной структуре
	return &pb.Token{Jwt: jwtToken}, nil
}

func createJWT(userID string) (string, error) {
	// Получение ключа подписи из переменных окружения
	key := []byte(os.Getenv("SIGNING_KEY"))

	// Получение текущего времени
	now := time.Now()

	// Создание нового JWT-токена с Claims:
	// - iss: издатель токена
	// - sub: идентификатор пользователя
	// - iat: время создания токена
	// - exp: время истечения токена (24 часа)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"iss": "auth-service",                 // Издатель
			"sub": userID,                         // ID пользователя
			"iat": now.Unix(),                     // Время создания
			"exp": now.Add(24 * time.Hour).Unix(), // Время истечения
		})

	// Подписание токена секретным ключом
	signedToken, err := token.SignedString(key)
	if err != nil {
		// В случае ошибки подписи - возврат внутренней ошибки и пустого токена
		return "", status.Error(codes.Internal, err.Error())
	}

	// Возврат подписанного токена
	return signedToken, nil
}

// ValidateToken - метод объекта Implementation для валидации JWT токена
// Назначение: проверить корректность предоставленного токена и вернуть информацию о пользователе
//
// Параметры:
//   - ctx: контекст выполнения запроса (может содержать тайм-аут, трассировку и т.д.)
//   - token: протобуферная структура, содержащая JWT токен
//
// Возвращает:
//   - *pb.User: структура пользователя с идентификатором при успешной валидации
//   - error: ошибка в случае невалидного токена или проблем с аутентификацией
func (this *Implementation) ValidateToken(ctx context.Context, token *pb.Token) (*pb.User, error) {
	// Извлечение ключа подписи из переменных окружения
	// SIGNING_KEY - секретный ключ, используемый для проверки подписи токена
	// Преобразование строки в байтовый массив для криптографических операций
	key := []byte(os.Getenv("SIGNING_KEY"))

	// Вызов функции валидации JWT с переданным токеном и ключом подписи
	// Функция проверяет целостность, срок действия и подпись токена
	// Возвращает идентификатор пользователя при успешной проверке
	userID, err := validateJWT(token.Jwt, key)
	if err != nil {
		// В случае ошибки валидации (просроченный или некорректный токен)
		// возвращаем nil и ошибку для дальнейшей обработки на стороне клиента
		return nil, err
	}

	// Создание и возврат протобуферной структуры пользователя
	// с идентификатором, извлеченным из валидного токена
	return &pb.User{UserID: userID}, nil
}

// validateJWT выполняет валидацию и проверку JWT токена
// Принимает:
//   - t: токен в виде строки
//   - signingKey: ключ для проверки подписи токена
//
// Возвращает:
//   - ID пользователя (Subject) при успешной валидации
//   - Ошибку в случае невалидного токена
func validateJWT(t string, signingKey []byte) (string, error) {
	// Пользовательская структура Claims для парсинга токена
	// Расширяет стандартные RegisteredClaims библиотеки jwt
	// Позволяет работать со стандартными полями токена (exp, iat, sub и др.)
	type MyClaims struct {
		jwt.RegisteredClaims
	}

	// Парсинг и проверка токена с использованием пользовательских Claims
	// jwt.ParseWithClaims выполняет полную валидацию:
	// - Проверка подписи
	// - Декодирование Claims
	// - Проверка целостности токена
	parsedToken, err := jwt.ParseWithClaims(t, &MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Анонимная функция возвращает ключ для проверки подписи токена
		// Используется криптографический ключ, которым был подписан токен
		return signingKey, nil
	})

	// Обработка ошибок валидации токена
	if err != nil {
		// Специфичная обработка истекшего токена
		if errors.Is(err, jwt.ErrTokenExpired) {
			// Возврат ошибки с кодом "не аутентифицирован"
			// и информативным сообщением о необходимости получения нового токена
			return "", status.Error(codes.Unauthenticated, "token expired, get new token")
		} else {
			// Для других ошибок (неверная подпись, некорректный формат) -
			// общая ошибка аутентификации
			return "", status.Error(codes.Unauthenticated, "unauthenticated")
		}
	}

	// Безопасное преобразование Claims к пользовательскому типу MyClaims
	// Проверка, что распарсенные Claims имеют корректный тип
	claims, ok := parsedToken.Claims.(*MyClaims)
	if !ok {
		// Если преобразование не удалось - возврат внутренней ошибки сервера
		// Может означать несоответствие структуры Claims
		return "", status.Error(codes.Internal, "claims type assertion failed")
	}

	// Извлечение и возврат ID пользователя (Subject) из Claims
	// Subject обычно содержит уникальный идентификатор пользователя
	return claims.RegisteredClaims.Subject, nil
}
