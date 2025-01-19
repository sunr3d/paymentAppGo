CREATE USER 'money_movement_user'@'%' IDENTIFIED BY 'MM123'; -- Создание юзера с доступом только с локалхоста для работы с БД

CREATE DATABASE money_movement; -- Создание новой для транзакций

GRANT ALL PRIVILEGES ON money_movement.* TO 'money_movement_user'@'%'; -- Выдача суперюзера на всю БД

USE money_movement; -- Переключаемся на БД

-- Создаем таблицу "кошелек":
CREATE TABLE wallet (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, -- Уникальный идентификатор с автоинкрементом
    user_id VARCHAR(255) NOT NULL UNIQUE, -- Идентификатор пользователя
    wallet_type VARCHAR(255) NOT NULL, -- Тип кошелька (CUSTOMER/MERCHANT)
    INDEX(user_id) -- Индексирование в таблице происходит по user_id // TODO: Пойми как работает индексация
);

-- Создаем таблицу счет:
CREATE TABLE account (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, -- Уникальный идентификатор с автоинкрементом
    cents INT NOT NULL DEFAULT 0, -- Баланс в центах, для точности фин. расчетов
    account_type VARCHAR(255) NOT NULL, -- Тип счета (DEFAULT/PAYMENT/INCOMING)
    wallet_id INT NOT NULL, -- Внешний ключ к идентификатору кошелька
    FOREIGN KEY (wallet_id) REFERENCES wallet(id)
);

-- Создание таблицы транзакций:
CREATE TABLE transaction (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, -- Уникальный идентификатор с автоинкрементом
    pid VARCHAR(255) NOT NULL, -- Идентификатор платежа
    src_user_id VARCHAR(255) NOT NULL, -- Идентификатор отправителя
    dst_user_id VARCHAR(255) NOT NULL, -- Идентификатор получателя
    src_wallet_id INT NOT NULL, -- Идентификатор кошелька отправителя
    dst_wallet_id INT NOT NULL, -- Идентификатор кошелька получателя
    src_account_type VARCHAR(255) NOT NULL, -- Тип аккаунта отправителя
    dst_account_type VARCHAR(255) NOT NULL, -- Тип аккаунта получателя
    final_dst_merchant_wallet_id INT, -- Опциональный идентификатор кошелька конечного продавца
    amount INT NOT NULL, -- Сумма транзакции в центах
    INDEX(pid) -- Индексирование по идентификатору платежа для быстрого поиска
);

-- Добавление "кошельков" продавцов и покупателей
INSERT INTO wallet(id, user_id, wallet_type) VALUES
    (1,'example@email.com', 'CUSTOMER'),
    (2, 'merchant_id', 'MERCHANT');

-- Добавление счета покупателей
INSERT INTO account(cents, account_type, wallet_id) VALUES
    (5000000, 'DEFAULT', 1), -- Основной счет покупателя
    (0, 'PAYMENT', 1); -- Счет для платежей покупателя

-- Добавление счета продавца
INSERT INTO account(cents, account_type, wallet_id) VALUES
    (0, 'INCOMING', 2); -- Счет для входяших платежей продавца
