-- Создаем юзера "auth_user" с разрешением подключения только с локалхоста, под паролем "Auth123"
-- (!!!СОЗДАВАТЬ ТАК АДМИН ЮЗЕРА НЕБЕЗОПАСНО!!!)
CREATE USER 'auth_user'@'%' IDENTIFIED BY 'Auth123';

-- Классическое создание БД под названием "auth"
CREATE DATABASE auth;

-- Выдаем супер-юзера на БД юзеру "auth_user" (помним про доступ только с локалхоста)
GRANT ALL PRIVILEGES ON auth.* TO 'auth_user'@'%';

-- Переключаемся на БД "auth"
USE auth;

-- Создаем таблицу "user" с колонками:
    -- "id" (ключевая, при каждом новом добавлении происходит +1)
    -- "user_id" (логин в виде e-mail, уникальный, не может быть пустой)
    -- "password" (пароль, не может быть пустой)
CREATE TABLE users (
    id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL
);

-- Добавляем первого юзера в таблицу "user" (id не добавляем, так как он автоматически инкрементируется)
INSERT INTO users (user_id, password) VALUES ('example@email.com', "ExamplePassword");

-- !!!В РЕАЛЬНОМ ПРОЕКТЕ ВСЕ ПАРОЛИ ДОЛЖНЫ БЫТЬ ХЕШИРОВАНЫ А НЕ ХРАНИТЬСЯ В ОТКРЫТОМ ВИДЕ!!!

