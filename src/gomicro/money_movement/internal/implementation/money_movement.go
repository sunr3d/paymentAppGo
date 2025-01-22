package mm

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid" // Пакет для работы со стрингами вида ID
	"github.com/sunr3d/gomicro/internal/producer"
	pb "github.com/sunr3d/gomicro/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	insertTransactionQuery = "INSERT INTO transaction (pid, src_user_id, dst_user_id, src_wallet_id, dst_wallet_id, src_account_type, dst_account_type, final_dst_merchant_wallet_id, amount) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	selectTransactionQuery = "SELECT id, pid, src_user_id, dst_user_id, src_wallet_id, dst_wallet_id, src_account_type, dst_account_type, final_dst_merchant_wallet_id, amount FROM transaction WHERE pid = ?"
)

// Implementation представляет сервис перемещения денежных средств.
// Реализует интерфейс MoneyMovementServiceServer
type Implementation struct {
	db *sql.DB
	pb.UnimplementedMoneyMovementServiceServer
}

// NewMoneyMovementImplementation создает новый экземпляр сервиса перемещения денег
//
// Параметры:
//   - db: подключение к базе данных
//
// Возвращает:
//   - указатель на новый экземпляр Implementation
func NewMoneyMovementImplementation(db *sql.DB) *Implementation {
	return &Implementation{db: db}
}

// Authorize выполняет авторизацию платежа
//
// Основные шаги:
//  1. Проверка валюты
//  2. Начало SQL транзакции
//  3. Получение кошельков покупателя и продавца
//  4. Перевод средств между счетами
//  5. Создание транзакции
//
// Параметры:
//   - ctx: контекст выполнения
//   - authorizePayload: данные для авторизации платежа
//
// Возвращает:
//   - идентификатор транзакции
//   - ошибку в случае неудачи
func (this *Implementation) Authorize(ctx context.Context, authorizePayload *pb.AuthorizePayload) (*pb.AuthorizeResponse, error) {
	// Проверка поддерживаемой валюты
	if authorizePayload.GetCurrency() != "USD" {
		return nil, status.Error(codes.InvalidArgument, "only accepts USD")
	}

	// Начало транзакции (включаем изолированный запрос)
	tx, err := this.db.Begin()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Получаем айди кошелька продавца
	merchantWallet, err := fetchWallet(tx, authorizePayload.MerchantWalletUserID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получаем айди кошелька покупателя
	customerWallet, err := fetchWallet(tx, authorizePayload.CustomerWalletUserID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получаем айди базового счета покупателя
	srcAccount, err := fetchAccount(tx, customerWallet.ID, "DEFAULT")
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получаем айди расчетного счета покупателя
	dstAccount, err := fetchAccount(tx, customerWallet.ID, "PAYMENT")
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Переводим деньги с базового на расчетный счет в количестве == платежу
	err = transfer(tx, srcAccount, dstAccount, authorizePayload.Cents)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Создаем айди транзакции для дальнейшей работы с ней
	pid := uuid.NewString()
	err = createTransaction(tx,
		pid,
		srcAccount,
		dstAccount,
		customerWallet,
		customerWallet,
		merchantWallet,
		authorizePayload.Cents)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Конец транзакции, коммит изменений в БД
	err = tx.Commit()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.AuthorizeResponse{Pid: pid}, nil
}

// Capture подтверждает ранее авторизованный платеж
//
// Основные шаги:
//  1. Начало SQL транзакции
//  2. Получение информации о предыдущей транзакции
//  3. Перевод средств на счет продавца
//  4. Создание новой транзакции
//
// Параметры:
//   - ctx: контекст выполнения
//   - capturePayload: данные для подтверждения платежа
//
// Возвращает:
//   - пустой ответ
//   - ошибку в случае неудачи
func (this *Implementation) Capture(ctx context.Context, capturePayload *pb.CapturePayload) (*emptypb.Empty, error) {
	// Начало транзакции (включаем изолированный запрос)
	tx, err := this.db.Begin()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Получение информации о транзакции по её pid
	authorizeTransaction, err := fetchTransaction(tx, capturePayload.Pid)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получение информации о расчетном счете
	srcAccount, err := fetchAccount(tx, authorizeTransaction.dstAccountWalletID, "PAYMENT")
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получение информации о счете продавца
	dstMerchantAccount, err := fetchAccount(tx, authorizeTransaction.finalDstMerchantWalletID, "INCOMING")
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получение айди кошелька клиента
	customerWallet, err := fetchWallet(tx, authorizeTransaction.srcUserID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Получение айди кошелька продавца
	merchantWallet, err := fetchWalletWithWalletID(tx, authorizeTransaction.finalDstMerchantWalletID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Перевод средств с расчетного счета клиента на расчетный счет продавца
	err = transfer(tx, srcAccount, dstMerchantAccount, authorizeTransaction.amount)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
		return nil, err
	}

	// Создание транзакции перевода средств от клиента продавцу
	err = createTransaction(
		tx,                          // БД
		authorizeTransaction.pid,    // айди транзакции
		srcAccount,                  // счет отправления
		dstMerchantAccount,          // счет получения
		customerWallet,              // кошелек отправителя
		merchantWallet,              // кошелек получателя
		merchantWallet,              // конечный кошелек получателя
		authorizeTransaction.amount) // сумма
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, status.Error(codes.Internal, rollbackErr.Error())
		}
	}

	// Запись транзакции в БД
	err = tx.Commit()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	producer.SendCaptureMessage(authorizeTransaction.pid, authorizeTransaction.srcUserID, authorizeTransaction.amount)

	return &emptypb.Empty{}, nil

}

func fetchWallet(tx *sql.Tx, userID string) (wallet, error) {
	var w wallet
	stmt, err := tx.Prepare("SELECT id, user_id, wallet_type FROM wallet WHERE user_id=?")
	if err != nil {
		return w, status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	err = stmt.QueryRow(userID).Scan(&w.ID, &w.userID, &w.walletType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return w, status.Error(codes.NotFound, err.Error())
		}
		return w, status.Error(codes.Internal, err.Error())
	}
	return w, nil
}

func fetchWalletWithWalletID(tx *sql.Tx, walletID int32) (wallet, error) {
	var w wallet
	stmt, err := tx.Prepare("SELECT id, user_id, wallet_type FROM wallet WHERE id=?")
	if err != nil {
		return w, status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	err = stmt.QueryRow(walletID).Scan(&w.ID, &w.userID, &w.walletType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return w, status.Error(codes.NotFound, err.Error())
		}
		return w, status.Error(codes.Internal, err.Error())
	}
	return w, nil
}

func fetchAccount(tx *sql.Tx, walletID int32, accountType string) (account, error) {
	var a account
	stmt, err := tx.Prepare("SELECT id, cents, account_type, wallet_id FROM account WHERE wallet_id=? AND account_type=?")
	if err != nil {
		return a, status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	err = stmt.QueryRow(walletID, accountType).Scan(&a.ID, &a.cents, &a.accountType, &a.walletID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, status.Error(codes.NotFound, err.Error())
		}
		return a, status.Error(codes.Internal, err.Error())
	}
	return a, nil
}

func transfer(tx *sql.Tx, srcAccount account, dstAccount account, amount int64) error {
	if srcAccount.cents < amount {
		return status.Error(codes.Aborted, "not enough money")
	}

	// SQL запрос для обновления счетов
	stmt, err := tx.Prepare("UPDATE account SET cents=? WHERE id=?")
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	// Снимаем деньги с базового аккаунта (srcAccount)
	_, err = stmt.Exec(srcAccount.cents-amount, srcAccount.ID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	// Перекидываем деньги на расчетный счет (dstAccount)
	_, err = stmt.Exec(dstAccount.cents+amount, dstAccount.ID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func createTransaction(tx *sql.Tx, pid string, srcAccount account, dstAccount account, srcWallet wallet, dstWallet wallet, finalDstWallet wallet, amount int64) error {

	// SQL запрос
	stmt, err := tx.Prepare(insertTransactionQuery)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		pid,                    // Уникальный айди транзакции
		srcWallet.userID,       // Айди кошелька отправителя
		dstWallet.userID,       // Айди кошелька получателя
		srcWallet.ID,           // Айди счета отправления
		dstWallet.ID,           // Айди счета получения
		srcAccount.accountType, // Тип счета отправления
		dstAccount.accountType, // Тип счета получения
		finalDstWallet.ID,      // Айди кошелька продавца
		amount)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	return nil
}

func fetchTransaction(tx *sql.Tx, pid string) (transaction, error) {
	var t transaction

	stmt, err := tx.Prepare(selectTransactionQuery)
	if err != nil {
		return t, status.Error(codes.Internal, err.Error())
	}
	defer stmt.Close()

	err = stmt.QueryRow(pid).Scan(
		&t.ID,
		&t.pid,
		&t.srcUserID,
		&t.dstUserID,
		&t.srcAccountWalletID,
		&t.dstAccountWalletID,
		&t.srcAccountType,
		&t.dstAccountType,
		&t.finalDstMerchantWalletID,
		&t.amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, status.Error(codes.NotFound, err.Error())
		}
		return t, status.Error(codes.Internal, err.Error())
	}

	return t, nil
}
