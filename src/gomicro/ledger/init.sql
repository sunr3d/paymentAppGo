CREATE USER 'ledger_user'@'%' IDENTIFIED BY 'Ledger123'; -- Создание юзера с доступом только с локалхоста для работы с БД

CREATE DATABASE ledger; -- Создание новой для транзакций

GRANT ALL PRIVILEGES ON ledger.* TO 'ledger_user'@'%'; -- Выдача суперюзера на всю БД

USE ledger; -- Переключаемся на БД

-- Создание таблицы транзакций:
CREATE TABLE transaction (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, -- Уникальный идентификатор с автоинкрементом
    order_id VARCHAR(255) NOT NULL, -- Идентификатор платежа
    user_id VARCHAR(255) NOT NULL, -- Идентификатор покупателя
    amount INT NOT NULL, -- Сумма транзакции в центах
    operation VARCHAR(255) NOT NULL, -- Название операции
    date DATE NOT NULL, -- Дата транзакции
    INDEX(order_id) -- Индексирование по идентификатору платежа для быстрого поиска
);


